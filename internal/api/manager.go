package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"debpub/internal/config"
	"debpub/internal/debian"
	"debpub/internal/storage"

	"github.com/dsnet/compress/bzip2"
	"github.com/ulikunitz/xz"
)

// RepositoryManager coordinates fetching and caching Debian repository indices.
type RepositoryManager struct {
	mu              sync.RWMutex
	cfg             *config.Config
	storage         storage.StorageBackend
	lastSync        time.Time
	indexes         map[string]*debian.Index // key: codename + "/" + component + "/" + arch
	knownCodenames  []string
	knownComponents []string
	knownArchs      []string
	releaseMeta     map[string]*debian.ReleaseManifest // key: codename
}

// NewRepositoryManager creates a new RepositoryManager.
func NewRepositoryManager(cfg *config.Config, backend storage.StorageBackend) *RepositoryManager {
	return &RepositoryManager{
		cfg:         cfg,
		storage:     backend,
		indexes:     make(map[string]*debian.Index),
		releaseMeta: make(map[string]*debian.ReleaseManifest),
	}
}

// SyncIndexes fetches indices for specified or configured codename and component.
func (m *RepositoryManager) SyncIndexes(ctx context.Context, codename, component string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if codename == "" {
		codename = m.cfg.Codename
	}
	if component == "" {
		component = m.cfg.Component
	}

	// If codename is still empty, attempt to auto-discover codenames in dists/
	var targetCodenames []string
	if codename != "" {
		targetCodenames = []string{codename}
	} else {
		// Discover codenames under dists
		entries, err := m.storage.List(ctx, "dists")
		if err != nil || len(entries) == 0 {
			entries, _ = m.storage.List(ctx, "dists/")
		}
		if len(entries) > 0 {
			discovered := make(map[string]bool)
			for _, item := range entries {
				cleanItem := strings.TrimPrefix(strings.TrimPrefix(item, "/"), "dists/")
				if cleanItem == item {
					cleanItem = strings.TrimPrefix(item, "dists")
					cleanItem = strings.TrimPrefix(cleanItem, "/")
				}
				parts := strings.Split(cleanItem, "/")
				if len(parts) > 0 && parts[0] != "" && parts[0] != ".lock" {
					discovered[parts[0]] = true
				}
			}
			for c := range discovered {
				targetCodenames = append(targetCodenames, c)
			}
			slices.Sort(targetCodenames)
		}
		if len(targetCodenames) == 0 {
			// Fallback to common Debian / Ubuntu codenames
			common := []string{"stable", "dev", "test", "prod", "bookworm", "bullseye", "trixie", "jammy", "noble"}
			for _, c := range common {
				if exists, _ := m.storage.Exists(ctx, path.Join("dists", c, "Release")); exists {
					targetCodenames = append(targetCodenames, c)
				}
			}
		}
	}

	if len(targetCodenames) == 0 && codename != "" {
		targetCodenames = []string{codename}
	}

	for _, curCodename := range targetCodenames {
		if !slices.Contains(m.knownCodenames, curCodename) {
			m.knownCodenames = append(m.knownCodenames, curCodename)
		}

		// 1. Try reading dists/<curCodename>/Release to discover components, architectures and metadata
		releasePath := path.Join("dists", curCodename, "Release")
		rc, err := m.storage.Get(ctx, releasePath)
		if err == nil {
			defer rc.Close()
			relBytes, errRead := io.ReadAll(rc)
			if errRead == nil {
				paras, errP := debian.ParseParagraphs(bytes.NewReader(relBytes))
				if errP == nil && len(paras) > 0 {
					p := paras[0]
					manifest := &debian.ReleaseManifest{
						Origin:      p.Get("Origin"),
						Label:       p.Get("Label"),
						Suite:       p.Get("Suite"),
						Codename:    p.Get("Codename"),
						Date:        p.Get("Date"),
						Description: p.Get("Description"),
					}
					if manifest.Codename == "" {
						manifest.Codename = curCodename
					}
					if archs := p.Get("Architectures"); archs != "" {
						manifest.Architectures = strings.Fields(archs)
					}
					if comps := p.Get("Components"); comps != "" {
						manifest.Components = strings.Fields(comps)
					}
					m.releaseMeta[curCodename] = manifest
					slog.Info("Discovered Release manifest", "codename", curCodename, "archs", manifest.Architectures, "comps", manifest.Components, "date", manifest.Date)

					// Update discovered components and architectures
					for _, comp := range manifest.Components {
						if !slices.Contains(m.knownComponents, comp) {
							m.knownComponents = append(m.knownComponents, comp)
						}
					}
					for _, arch := range manifest.Architectures {
						if !slices.Contains(m.knownArchs, arch) {
							m.knownArchs = append(m.knownArchs, arch)
						}
					}
				}
			}
		}

		// Determine architectures to fetch
		archs := m.cfg.Architectures
		if len(archs) == 0 {
			if manifest, ok := m.releaseMeta[curCodename]; ok && len(manifest.Architectures) > 0 {
				archs = manifest.Architectures
			}
		}
		if len(archs) == 0 {
			archs = []string{"all", "arm64", "amd64", "armhf", "i386"}
		}

		// Determine components to fetch
		comps := []string{component}
		if manifest, ok := m.releaseMeta[curCodename]; ok && len(manifest.Components) > 0 {
			for _, comp := range manifest.Components {
				if !slices.Contains(comps, comp) {
					comps = append(comps, comp)
				}
			}
		}

		for _, comp := range comps {
			if comp != "" && !slices.Contains(m.knownComponents, comp) {
				m.knownComponents = append(m.knownComponents, comp)
			}
			for _, arch := range archs {
				binaryArch := "binary-" + arch
				key := curCodename + "/" + comp + "/" + arch

				rawIndex, err := m.fetchPackagesIndexData(ctx, curCodename, comp, binaryArch)
				if err != nil {
					continue
				}

				parsed, err := debian.ParseIndex(rawIndex)
				if err != nil {
					slog.Warn("Failed to parse index", "codename", curCodename, "comp", comp, "arch", arch, "err", err)
					continue
				}

				m.indexes[key] = parsed
				slog.Info("Successfully indexed Packages", "codename", curCodename, "comp", comp, "arch", arch, "packages", len(parsed.Packages))
				if !slices.Contains(m.knownArchs, arch) {
					m.knownArchs = append(m.knownArchs, arch)
				}
			}
		}
	}

	m.lastSync = time.Now()
	return nil
}

// fetchPackagesIndexData attempts to fetch Packages, Packages.xz, Packages.gz, or Packages.bz2.
func (m *RepositoryManager) fetchPackagesIndexData(ctx context.Context, codename, component, binaryArch string) ([]byte, error) {
	base := path.Join("dists", codename, component, binaryArch, "Packages")
	variants := []string{
		"",     // plain Packages
		".gz",  // Packages.gz
		".xz",  // Packages.xz
		".bz2", // Packages.bz2
	}

	for _, ext := range variants {
		targetPath := base + ext
		rc, err := m.storage.Get(ctx, targetPath)
		if err != nil {
			slog.Debug("Target index variant not found or error", "path", targetPath, "err", err)
			continue
		}
		defer rc.Close()

		compressed, err := io.ReadAll(rc)
		if err != nil {
			slog.Warn("Failed reading index variant stream", "path", targetPath, "err", err)
			continue
		}

		decompressed, err := decompressData(ext, compressed)
		if err == nil {
			slog.Info("Fetched index variant from storage", "path", targetPath, "bytes", len(decompressed))
			return decompressed, nil
		}
		slog.Warn("Failed decompressing index variant", "path", targetPath, "ext", ext, "err", err)
	}

	return nil, storage.ErrNotFound
}

func decompressData(ext string, data []byte) ([]byte, error) {
	switch ext {
	case "":
		return data, nil
	case ".gz":
		gr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		defer gr.Close()
		return io.ReadAll(gr)
	case ".bz2":
		br, err := bzip2.NewReader(bytes.NewReader(data), nil)
		if err != nil {
			return nil, err
		}
		defer br.Close()
		return io.ReadAll(br)
	case ".xz":
		xr, err := xz.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		return io.ReadAll(xr)
	default:
		return nil, fmt.Errorf("unsupported compression: %s", ext)
	}
}

// GetRepoInfo returns current status and metadata of the repository.
func (m *RepositoryManager) GetRepoInfo(codename, component string) RepoInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if codename == "" {
		codename = m.cfg.Codename
	}
	if codename == "" && len(m.knownCodenames) > 0 {
		codename = m.knownCodenames[0]
	}
	if component == "" {
		component = m.cfg.Component
	}

	info := RepoInfo{
		Storage:        m.cfg.Storage,
		Bucket:         m.cfg.Bucket,
		Prefix:         m.cfg.Prefix,
		LocalDir:       m.cfg.LocalDir,
		Codename:       codename,
		Component:      component,
		Architectures:  m.knownArchs,
		AllCodenames:   m.knownCodenames,
		AllComponents:  m.knownComponents,
		LastSyncedTime: m.lastSync,
	}

	if rel, ok := m.releaseMeta[codename]; ok {
		info.ReleaseDate = rel.Date
		info.Origin = rel.Origin
		info.Label = rel.Label
		info.Description = rel.Description
	}

	allStanzas := m.collectStanzas(codename, component, "")
	pkgsMap := make(map[string]bool)
	for _, s := range allStanzas {
		pkgsMap[s.Package] = true
	}
	info.TotalPackages = len(pkgsMap)
	info.TotalVersions = len(allStanzas)

	return info
}

// ListPackages returns package cards matching the query parameters.
func (m *RepositoryManager) ListPackages(codename, component, arch, query string) []PackageCard {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stanzas := m.collectStanzas(codename, component, arch)

	// Group stanzas by package name
	grouped := make(map[string][]*debian.PackageStanza)
	for _, s := range stanzas {
		grouped[s.Package] = append(grouped[s.Package], s)
	}

	query = strings.ToLower(strings.TrimSpace(query))
	cards := make([]PackageCard, 0)

	for name, list := range grouped {
		if query != "" {
			matched := strings.Contains(strings.ToLower(name), query)
			if !matched {
				for _, s := range list {
					if strings.Contains(strings.ToLower(s.Description), query) {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
		}

		// Sort versions descending (latest first)
		slices.SortFunc(list, func(a, b *debian.PackageStanza) int {
			return debian.CompareVersions(b.Version, a.Version)
		})

		latest := list[0]
		var archs []string
		for _, s := range list {
			if !slices.Contains(archs, s.Architecture) {
				archs = append(archs, s.Architecture)
			}
		}

		cards = append(cards, PackageCard{
			Name:           name,
			LatestVersion:  latest.Version,
			Architectures:  archs,
			VersionCount:   len(list),
			Description:    latest.Description,
			Maintainer:     latest.Maintainer,
			LatestSize:     latest.Size,
			LatestFilename: latest.Filename,
			LatestSHA256:   latest.SHA256,
		})
	}

	// Sort cards alphabetically by package name
	slices.SortFunc(cards, func(a, b PackageCard) int {
		return strings.Compare(a.Name, b.Name)
	})

	return cards
}

// GetPackageDetail returns detailed information for all versions of a package.
func (m *RepositoryManager) GetPackageDetail(codename, component, name string) (*PackageDetailResponse, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stanzas := m.collectStanzas(codename, component, "")
	var versions []PackageVersionDetail

	for _, s := range stanzas {
		if s.Package != name {
			continue
		}
		versions = append(versions, PackageVersionDetail{
			Package:       s.Package,
			Version:       s.Version,
			Architecture:  s.Architecture,
			Maintainer:    s.Maintainer,
			InstalledSize: s.InstalledSize,
			Section:       s.Section,
			Priority:      s.Priority,
			Depends:       s.Depends,
			PreDepends:    s.PreDepends,
			Recommends:    s.Recommends,
			Suggests:      s.Suggests,
			Conflicts:     s.Conflicts,
			Breaks:        s.Breaks,
			Replaces:      s.Replaces,
			Provides:      s.Provides,
			Description:   s.Description,
			Homepage:      s.Homepage,
			Filename:      s.Filename,
			Size:          s.Size,
			SHA256:        s.SHA256,
			SHA512:        s.SHA512,
			SHA1:          s.SHA1,
			MD5sum:        s.MD5sum,
			CustomFields:  s.CustomFields,
		})
	}

	if len(versions) == 0 {
		return nil, false
	}

	// Sort versions descending
	slices.SortFunc(versions, func(a, b PackageVersionDetail) int {
		return debian.CompareVersions(b.Version, a.Version)
	})

	return &PackageDetailResponse{
		Name:     name,
		Versions: versions,
	}, true
}

func (m *RepositoryManager) collectStanzas(codename, component, arch string) []*debian.PackageStanza {
	if codename == "" {
		codename = m.cfg.Codename
	}
	if codename == "" && len(m.knownCodenames) > 0 {
		codename = m.knownCodenames[0]
	}
	if component == "" {
		component = m.cfg.Component
	}

	prefix := ""
	if codename != "" && component != "" {
		prefix = codename + "/" + component + "/"
	} else if codename != "" {
		prefix = codename + "/"
	}
	var results []*debian.PackageStanza
	seenKey := make(map[string]bool)

	for key, idx := range m.indexes {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		keyArch := strings.TrimPrefix(key, prefix)
		if arch != "" && keyArch != arch {
			continue
		}

		for _, s := range idx.Packages {
			ident := s.Package + "_" + s.Version + "_" + s.Architecture
			if !seenKey[ident] {
				seenKey[ident] = true
				results = append(results, s)
			}
		}
	}

	return results
}
