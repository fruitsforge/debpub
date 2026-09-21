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
				// Only consider real subdirectories under dists/ (depth >= 2, no dot extensions or hidden files)
				if len(parts) >= 2 && parts[0] != "" && !strings.HasPrefix(parts[0], ".") && !strings.Contains(parts[0], ".") {
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

		// 1. Try reading dists/<curCodename>/InRelease or dists/<curCodename>/Release
		var relBytes []byte
		for _, relCandidate := range []string{
			path.Join("dists", curCodename, "InRelease"),
			path.Join("dists", curCodename, "Release"),
		} {
			rc, err := m.storage.Get(ctx, relCandidate)
			if err == nil {
				b, errRead := io.ReadAll(rc)
				_ = rc.Close()
				if errRead == nil && len(b) > 0 {
					relBytes = b
					break
				}
			}
		}

		if len(relBytes) > 0 {
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

		// Determine architectures and components to search for
		archs := m.cfg.Architectures
		if len(archs) == 0 {
			if manifest, ok := m.releaseMeta[curCodename]; ok && len(manifest.Architectures) > 0 {
				archs = manifest.Architectures
			}
		}
		if len(archs) == 0 {
			archs = []string{"all", "arm64", "amd64", "armhf", "i386"}
		}

		comps := []string{component}
		if manifest, ok := m.releaseMeta[curCodename]; ok && len(manifest.Components) > 0 {
			for _, comp := range manifest.Components {
				if !slices.Contains(comps, comp) {
					comps = append(comps, comp)
				}
			}
		}

		// 2. Discover and ingest Packages indexes (both hierarchical and flat repository layouts)
		seenFiles := make(map[string]bool)

		// First, check storage listing under dists/<curCodename>
		if listed, err := m.storage.List(ctx, path.Join("dists", curCodename)); err == nil {
			for _, item := range listed {
				base := path.Base(item)
				if strings.HasPrefix(base, "Packages") && !seenFiles[item] {
					seenFiles[item] = true
					if rawIndex, err := m.fetchIndexFile(ctx, item); err == nil {
						if parsed, err := debian.ParseIndex(rawIndex); err == nil {
							m.ingestIndex(curCodename, item, parsed, component)
						} else {
							slog.Warn("Failed to parse index", "path", item, "err", err)
						}
					}
				}
			}
		}

		// Second, probe standard hierarchical and flat paths in case storage listing didn't discover them
		variants := []string{"", ".gz", ".xz", ".bz2"}
		var probePaths []string

		// Probing flat paths: dists/<curCodename>/Packages(.ext)
		for _, ext := range variants {
			probePaths = append(probePaths, path.Join("dists", curCodename, "Packages"+ext))
		}

		// Probing hierarchical paths: dists/<curCodename>/<comp>/binary-<arch>/Packages(.ext)
		for _, comp := range comps {
			if comp != "" && !slices.Contains(m.knownComponents, comp) {
				m.knownComponents = append(m.knownComponents, comp)
			}
			for _, arch := range archs {
				binaryArch := "binary-" + arch
				for _, ext := range variants {
					probePaths = append(probePaths, path.Join("dists", curCodename, comp, binaryArch, "Packages"+ext))
				}
			}
		}

		for _, p := range probePaths {
			if seenFiles[p] {
				continue
			}
			if rawIndex, err := m.fetchIndexFile(ctx, p); err == nil {
				seenFiles[p] = true
				if parsed, err := debian.ParseIndex(rawIndex); err == nil {
					m.ingestIndex(curCodename, p, parsed, component)
				} else {
					slog.Warn("Failed to parse index", "path", p, "err", err)
				}
			}
		}
	}

	m.lastSync = time.Now()
	return nil
}

// ingestIndex partitions and stores packages from an index into m.indexes by architecture and component.
func (m *RepositoryManager) ingestIndex(curCodename, targetPath string, parsed *debian.Index, defaultComp string) {
	if defaultComp == "" {
		defaultComp = m.cfg.Component
	}
	if defaultComp == "" {
		defaultComp = "main"
	}

	for _, stanza := range parsed.Packages {
		arch := stanza.Architecture
		if arch == "" {
			for _, part := range strings.Split(targetPath, "/") {
				if strings.HasPrefix(part, "binary-") {
					arch = strings.TrimPrefix(part, "binary-")
					break
				}
			}
		}
		if arch == "" {
			arch = "all"
		}

		comp := ""
		// Extract component from directory path if structured (dists/<codename>/<comp>/binary-<arch>/...)
		relToCodename := strings.TrimPrefix(targetPath, path.Join("dists", curCodename)+"/")
		parts := strings.Split(relToCodename, "/")
		if len(parts) >= 2 && !strings.HasPrefix(parts[0], "binary-") && !strings.HasPrefix(parts[0], "Packages") {
			comp = parts[0]
		}

		// Infer component from package stanza Filename or Section
		if comp == "" {
			if strings.HasPrefix(stanza.Filename, "pool/") {
				poolParts := strings.Split(strings.TrimPrefix(stanza.Filename, "pool/"), "/")
				if len(poolParts) > 1 && poolParts[0] != "" {
					comp = poolParts[0]
				}
			} else if strings.Contains(stanza.Section, "/") {
				comp = strings.Split(stanza.Section, "/")[0]
			}
		}

		if comp == "" {
			comp = defaultComp
		}

		if !slices.Contains(m.knownComponents, comp) {
			m.knownComponents = append(m.knownComponents, comp)
		}
		if !slices.Contains(m.knownArchs, arch) {
			m.knownArchs = append(m.knownArchs, arch)
		}

		key := curCodename + "/" + comp + "/" + arch
		idx, exists := m.indexes[key]
		if !exists {
			idx = debian.NewIndex()
			m.indexes[key] = idx
		}
		idx.AddOrUpdate(stanza, true)
	}

	slog.Info("Ingested Packages index", "codename", curCodename, "path", targetPath, "packages", len(parsed.Packages))
}

// fetchIndexFile retrieves and decompresses an index file from storage.
func (m *RepositoryManager) fetchIndexFile(ctx context.Context, targetPath string) ([]byte, error) {
	rc, err := m.storage.Get(ctx, targetPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	ext := path.Ext(targetPath)
	return decompressData(ext, data)
}

// fetchPackagesIndexData attempts to fetch Packages, Packages.xz, Packages.gz, or Packages.bz2.
func (m *RepositoryManager) fetchPackagesIndexData(ctx context.Context, codename, component, binaryArch string) ([]byte, error) {
	base := path.Join("dists", codename, component, binaryArch, "Packages")
	variants := []string{"", ".gz", ".xz", ".bz2"}

	for _, ext := range variants {
		targetPath := base + ext
		decompressed, err := m.fetchIndexFile(ctx, targetPath)
		if err == nil {
			return decompressed, nil
		}
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
		out, err := io.ReadAll(gr)
		_ = gr.Close()
		return out, err
	case ".bz2":
		br, err := bzip2.NewReader(bytes.NewReader(data), nil)
		if err != nil {
			return nil, err
		}
		out, err := io.ReadAll(br)
		_ = br.Close()
		return out, err
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
