package repo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"slices"
	"strings"

	"debpub/internal/compress"
	"debpub/internal/config"
	"debpub/internal/debian"
	"debpub/internal/lock"
	"debpub/internal/storage"
)

// Publisher coordinates the Debian repository publishing lifecycle.
type Publisher struct {
	cfg     *config.Config
	storage storage.StorageBackend
	signer  debian.Signer
	locker  *lock.Locker
}

// NewPublisher initializes a new Publisher engine.
func NewPublisher(cfg *config.Config, backend storage.StorageBackend, signer debian.Signer) *Publisher {
	var locker *lock.Locker
	if cfg.LockEnabled {
		locker = lock.NewLocker(lock.LockerOptions{
			Backend:  backend,
			Codename: cfg.Codename,
			Timeout:  cfg.LockTimeout,
			TTL:      cfg.LockTTL,
			Reason:   "debpub publishing",
			Metadata: cfg.ExtraMetadata,
		})
	}

	return &Publisher{
		cfg:     cfg,
		storage: backend,
		signer:  signer,
		locker:  locker,
	}
}

// PublishDebFiles publishes one or more .deb package files into the repository using the 4-phase staged protocol.
func (p *Publisher) PublishDebFiles(ctx context.Context, debFilePaths []string) error {
	if len(debFilePaths) == 0 {
		return fmt.Errorf("no deb files provided to publish")
	}

	// 1. Inspect all candidate packages locally before acquiring lock
	var candidates []*debian.DebPackage
	for _, fp := range debFilePaths {
		f, err := os.Open(fp)
		if err != nil {
			return fmt.Errorf("failed opening candidate deb %s: %w", fp, err)
		}
		pkg, err := debian.ParseDebReader(f)
		_ = f.Close()
		if err != nil {
			return fmt.Errorf("failed parsing deb %s: %w", fp, err)
		}
		candidates = append(candidates, pkg)
		slog.Info("Inspected candidate Debian package", "package", pkg.Control.Package, "version", pkg.Control.Version, "arch", pkg.Control.Architecture)
	}

	// 2. Acquire distributed lock
	if p.locker != nil {
		if err := p.locker.Acquire(ctx); err != nil {
			return fmt.Errorf("failed acquiring repository lock: %w", err)
		}
		defer func() {
			_ = p.locker.Release(context.Background())
		}()
	}

	// 3. Phase 1: Payload Upload (Pool binary files)
	var uploadedPoolPaths []string
	for i, pkg := range candidates {
		poolPath := p.computePoolPath(pkg)
		debPath := debFilePaths[i]

		f, err := os.Open(debPath)
		if err != nil {
			return fmt.Errorf("phase 1: failed opening deb file %s: %w", debPath, err)
		}
		fi, err := f.Stat()
		if err != nil {
			_ = f.Close()
			return fmt.Errorf("phase 1: failed stat deb file %s: %w", debPath, err)
		}

		slog.Info("Phase 1: Uploading package binary to pool", "path", poolPath, "size", fi.Size())
		err = p.storage.Put(ctx, poolPath, f, fi.Size(), "application/vnd.debian.binary-package")
		_ = f.Close()
		if err != nil {
			return fmt.Errorf("phase 1 payload upload failed for %s: %w", poolPath, err)
		}
		uploadedPoolPaths = append(uploadedPoolPaths, poolPath)
	}

	// 4. Group candidates by Architecture
	archGroups := make(map[string][]*debian.PackageStanza)
	for i, pkg := range candidates {
		arch := pkg.Control.Architecture
		stanza := &debian.PackageStanza{
			PackageControl: pkg.Control,
			Filename:       uploadedPoolPaths[i],
			Size:           pkg.Size,
			SHA256:         pkg.SHA256,
			SHA512:         pkg.SHA512,
			SHA1:           pkg.SHA1,
			MD5sum:         pkg.MD5,
		}
		archGroups[arch] = append(archGroups[arch], stanza)
	}

	// 5. Phase 2: Index Updates & Compression
	var allIndexChecksums []debian.IndexChecksum
	modifiedArchs := make(map[string]bool)

	for arch, stanzas := range archGroups {
		modifiedArchs[arch] = true
		indexPath := path.Join("dists", p.cfg.Codename, p.cfg.Component, fmt.Sprintf("binary-%s", arch), "Packages")

		// Fetch existing index if available
		index := debian.NewIndex()
		rc, err := p.storage.Get(ctx, indexPath)
		if err == nil {
			existingData, errRead := io.ReadAll(rc)
			rc.Close()
			if errRead == nil {
				if parsedIdx, errParse := debian.ParseIndex(existingData); errParse == nil {
					index = parsedIdx
				}
			}
		} else if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("phase 2: error reading existing index %s: %w", indexPath, err)
		}

		// Merge candidates
		for _, s := range stanzas {
			index.AddOrUpdate(s, p.cfg.PreserveVersions)
		}

		rawIndex := index.Serialize()
		compressedRes, err := compress.CompressAll(rawIndex)
		if err != nil {
			return fmt.Errorf("phase 2: compression failed for %s: %w", indexPath, err)
		}

		// Upload all compression variants (Packages, Packages.gz, Packages.bz2, Packages.xz)
		baseDir := path.Join("dists", p.cfg.Codename, p.cfg.Component, fmt.Sprintf("binary-%s", arch))
		relPathBase := path.Join(p.cfg.Component, fmt.Sprintf("binary-%s", arch), "Packages")

		for _, out := range compressedRes.AllOutputs() {
			variantStoragePath := baseDir + "/Packages" + out.Ext
			variantRelPath := relPathBase + out.Ext

			slog.Info("Phase 2: Uploading index variant", "path", variantStoragePath, "size", out.Size)
			err := p.storage.PutBytes(ctx, variantStoragePath, out.Data, "text/plain")
			if err != nil {
				return fmt.Errorf("phase 2: failed uploading index variant %s: %w", variantStoragePath, err)
			}

			allIndexChecksums = append(allIndexChecksums, debian.IndexChecksum{
				Path:   variantRelPath,
				Size:   out.Size,
				MD5:    out.MD5,
				SHA1:   out.SHA1,
				SHA256: out.SHA256,
				SHA512: out.SHA512,
			})
		}
	}

	// 6. Phase 3: Atomic Release Manifest Activation
	var allArchs []string
	for a := range modifiedArchs {
		allArchs = append(allArchs, a)
	}
	for _, a := range p.cfg.Architectures {
		if !slices.Contains(allArchs, a) {
			allArchs = append(allArchs, a)
		}
	}

	manifest := &debian.ReleaseManifest{
		Origin:        p.cfg.Origin,
		Label:         p.cfg.Label,
		Suite:         p.cfg.Suite,
		Codename:      p.cfg.Codename,
		Architectures: allArchs,
		Components:    []string{p.cfg.Component},
		Description:   p.cfg.Description,
		Indices:       allIndexChecksums,
	}

	releaseData := debian.GenerateRelease(manifest)
	releasePath := path.Join("dists", p.cfg.Codename, "Release")

	slog.Info("Phase 3: Publishing Release manifest", "path", releasePath)
	if err := p.storage.PutBytes(ctx, releasePath, releaseData, "text/plain"); err != nil {
		return fmt.Errorf("phase 3: failed uploading Release: %w", err)
	}

	// GPG Signing (Dual manifest: InRelease + Release.gpg)
	if p.signer != nil && p.cfg.Sign {
		slog.Info("Phase 3: Signing Release manifest with GPG")

		// 1. Detached signature (Release.gpg)
		sigData, err := p.signer.SignDetached(ctx, releaseData)
		if err != nil {
			return fmt.Errorf("failed creating detached Release.gpg signature: %w", err)
		}
		gpgPath := path.Join("dists", p.cfg.Codename, "Release.gpg")
		if err := p.storage.PutBytes(ctx, gpgPath, sigData, "application/pgp-signature"); err != nil {
			return fmt.Errorf("failed uploading Release.gpg: %w", err)
		}

		// 2. Inline clearsigned (InRelease)
		inReleaseData, err := p.signer.SignClear(ctx, releaseData)
		if err != nil {
			return fmt.Errorf("failed creating inline InRelease signature: %w", err)
		}
		inReleasePath := path.Join("dists", p.cfg.Codename, "InRelease")
		if err := p.storage.PutBytes(ctx, inReleasePath, inReleaseData, "text/plain"); err != nil {
			return fmt.Errorf("failed uploading InRelease: %w", err)
		}
	}

	slog.Info("Repository publishing completed successfully!", "codename", p.cfg.Codename, "packages", len(candidates))
	return nil
}

// computePoolPath generates canonical Debian pool storage path: pool/<component>/<prefix>/<source>/<filename>
func (p *Publisher) computePoolPath(pkg *debian.DebPackage) string {
	pkgName := pkg.Control.Package
	firstLetter := string(pkgName[0])
	prefix := firstLetter
	if strings.HasPrefix(pkgName, "lib") && len(pkgName) > 3 {
		prefix = pkgName[:4]
	}

	filename := fmt.Sprintf("%s_%s_%s.deb", pkg.Control.Package, pkg.Control.Version, pkg.Control.Architecture)
	return path.Join("pool", p.cfg.Component, prefix, pkgName, filename)
}
