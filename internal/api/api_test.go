package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"debpub/internal/config"
	"debpub/internal/debian"
	"debpub/internal/storage"
)

type mockStorage struct {
	storage.StorageBackend
	files map[string][]byte
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		files: make(map[string][]byte),
	}
}

func (m *mockStorage) Get(_ context.Context, path string) (io.ReadCloser, error) {
	data, ok := m.files[path]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *mockStorage) Put(_ context.Context, path string, data io.Reader, _ int64, _ string) error {
	b, err := io.ReadAll(data)
	if err != nil {
		return err
	}
	m.files[path] = b
	return nil
}

func (m *mockStorage) PutBytes(_ context.Context, path string, data []byte, _ string) error {
	m.files[path] = data
	return nil
}

func (m *mockStorage) Exists(_ context.Context, path string) (bool, error) {
	_, ok := m.files[path]
	return ok, nil
}

func (m *mockStorage) List(_ context.Context, prefix string) ([]string, error) {
	var matches []string
	cleanPrefix := strings.TrimPrefix(prefix, "/")
	for p := range m.files {
		cleanP := strings.TrimPrefix(p, "/")
		if cleanPrefix == "" || strings.HasPrefix(cleanP, cleanPrefix) {
			matches = append(matches, cleanP)
		}
	}
	return matches, nil
}

func TestAPIServer(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Codename = "bookworm"
	cfg.Component = "main"
	cfg.Architectures = []string{"arm64"}

	mockStore := newMockStorage()

	// Seed mock storage with a Packages index
	idx := debian.NewIndex()
	idx.AddOrUpdate(&debian.PackageStanza{
		PackageControl: debian.PackageControl{
			Package:       "sample-app",
			Version:       "1.2.0-1",
			Architecture:  "arm64",
			Maintainer:    "Debpub Team <team@example.com>",
			InstalledSize: 1024,
			Section:       "utils",
			Priority:      "optional",
			Description:   "Sample utility application",
			Depends:       "libc6 (>= 2.34)",
		},
		Filename: "pool/main/s/sample-app/sample-app_1.2.0-1_arm64.deb",
		Size:     51200,
		SHA256:   "abcdef1234567890",
	}, true)

	// Add an older version of sample-app
	idx.AddOrUpdate(&debian.PackageStanza{
		PackageControl: debian.PackageControl{
			Package:       "sample-app",
			Version:       "1.1.0-1",
			Architecture:  "arm64",
			Maintainer:    "Debpub Team <team@example.com>",
			InstalledSize: 1000,
			Section:       "utils",
			Priority:      "optional",
			Description:   "Sample utility application",
		},
		Filename: "pool/main/s/sample-app/sample-app_1.1.0-1_arm64.deb",
		Size:     50000,
		SHA256:   "1122334455667788",
	}, true)

	// Add second package
	idx.AddOrUpdate(&debian.PackageStanza{
		PackageControl: debian.PackageControl{
			Package:       "other-tool",
			Version:       "2.0.0",
			Architecture:  "arm64",
			Maintainer:    "Other Team <other@example.com>",
			InstalledSize: 2048,
			Description:   "Other helper tool",
		},
		Filename: "pool/main/o/other-tool/other-tool_2.0.0_arm64.deb",
		Size:     102400,
		SHA256:   "aabbccddeeff0011",
	}, true)

	mockStore.files["dists/bookworm/main/binary-arm64/Packages"] = idx.Serialize()

	mgr := NewRepositoryManager(cfg, mockStore)
	server := NewServer(mgr)

	// 1. Test GET / (UI index page)
	t.Run("GET /", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte("debpub")) {
			t.Fatalf("expected response to contain 'debpub'")
		}
	})

	// 2. Test POST /api/fetch (Sync indexes)
	t.Run("POST /api/fetch", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/fetch", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp FetchResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode fetch response: %v", err)
		}
		if !resp.Success {
			t.Fatalf("expected success = true")
		}
		if resp.PackagesCount != 2 {
			t.Errorf("expected 2 packages, got %d", resp.PackagesCount)
		}
	})

	// 3. Test GET /api/info
	t.Run("GET /api/info", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/info", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var info RepoInfo
		if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
			t.Fatalf("failed to decode info: %v", err)
		}
		if info.Codename != "bookworm" {
			t.Errorf("expected codename = bookworm, got %s", info.Codename)
		}
		if info.TotalPackages != 2 {
			t.Errorf("expected 2 total packages, got %d", info.TotalPackages)
		}
		if info.TotalVersions != 3 {
			t.Errorf("expected 3 total versions, got %d", info.TotalVersions)
		}
	})

	// 4. Test GET /api/packages
	t.Run("GET /api/packages", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/packages", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var cards []PackageCard
		if err := json.Unmarshal(rec.Body.Bytes(), &cards); err != nil {
			t.Fatalf("failed to decode cards: %v", err)
		}
		if len(cards) != 2 {
			t.Fatalf("expected 2 cards, got %d", len(cards))
		}

		// Verify sample-app latest version and version count
		var sampleCard *PackageCard
		for i := range cards {
			if cards[i].Name == "sample-app" {
				sampleCard = &cards[i]
				break
			}
		}
		if sampleCard == nil {
			t.Fatalf("sample-app not found in cards")
		}
		if sampleCard.LatestVersion != "1.2.0-1" {
			t.Errorf("expected latest version 1.2.0-1, got %s", sampleCard.LatestVersion)
		}
		if sampleCard.VersionCount != 2 {
			t.Errorf("expected 2 versions, got %d", sampleCard.VersionCount)
		}
	})

	// 5. Test GET /api/packages with query filter
	t.Run("GET /api/packages?q=other", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/packages?q=other", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var cards []PackageCard
		if err := json.Unmarshal(rec.Body.Bytes(), &cards); err != nil {
			t.Fatalf("failed to decode cards: %v", err)
		}
		if len(cards) != 1 || cards[0].Name != "other-tool" {
			t.Fatalf("expected 1 result matching 'other-tool', got %d", len(cards))
		}
	})

	// 6. Test GET /api/packages/{name}
	t.Run("GET /api/packages/sample-app", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/packages/sample-app", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var detail PackageDetailResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
			t.Fatalf("failed to decode detail: %v", err)
		}
		if detail.Name != "sample-app" {
			t.Errorf("expected name sample-app, got %s", detail.Name)
		}
		if len(detail.Versions) != 2 {
			t.Fatalf("expected 2 versions, got %d", len(detail.Versions))
		}
		if detail.Versions[0].Version != "1.2.0-1" {
			t.Errorf("expected first version 1.2.0-1, got %s", detail.Versions[0].Version)
		}
		if detail.Versions[0].Depends != "libc6 (>= 2.34)" {
			t.Errorf("expected depends 'libc6 (>= 2.34)', got %s", detail.Versions[0].Depends)
		}
	})

	// 7. Test GET /api/packages/nonexistent (404)
	t.Run("GET /api/packages/nonexistent", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/packages/nonexistent", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 NotFound, got %d", rec.Code)
		}
	})
}

func TestFlatRepositoryLayoutAndDiscovery(t *testing.T) {
	mockStore := newMockStorage()

	// Seed flat Packages.gz index under dists/stable/Packages.gz
	idx := debian.NewIndex()
	idx.AddOrUpdate(&debian.PackageStanza{
		PackageControl: debian.PackageControl{
			Package:      "sample-flat",
			Version:      "1.0.0",
			Architecture: "all",
			Maintainer:   "Flat Team <flat@example.com>",
			Description:  "Sample flat package",
		},
		Filename: "pool/main/s/sample-flat/sample-flat_1.0.0_all.deb",
		Size:     12345,
		SHA256:   "abcdef",
	}, true)

	// Compress to .gz
	var gzBuf bytes.Buffer
	gw := gzip.NewWriter(&gzBuf)
	_, _ = gw.Write(idx.Serialize())
	_ = gw.Close()

	mockStore.files["dists/stable/Packages.gz"] = gzBuf.Bytes()
	// Loose file inside dists/ that should NOT be detected as a codename
	mockStore.files["dists/debian-dists-stable-Release.txt"] = []byte("Origin: Debian\n")

	cfg := config.DefaultConfig()
	mgr := NewRepositoryManager(cfg, mockStore)

	ctx := context.Background()
	// Auto-discovery without specifying codename
	if err := mgr.SyncIndexes(ctx, "", ""); err != nil {
		t.Fatalf("SyncIndexes failed: %v", err)
	}

	info := mgr.GetRepoInfo("stable", "main")
	if len(info.AllCodenames) != 1 || info.AllCodenames[0] != "stable" {
		t.Fatalf("expected only 'stable' codename discovered, got %v", info.AllCodenames)
	}

	if info.TotalPackages != 1 {
		t.Fatalf("expected 1 package from flat Packages.gz, got %d", info.TotalPackages)
	}

	cards := mgr.ListPackages("stable", "main", "all", "")
	if len(cards) != 1 || cards[0].Name != "sample-flat" {
		t.Fatalf("expected sample-flat card, got %+v", cards)
	}
}
