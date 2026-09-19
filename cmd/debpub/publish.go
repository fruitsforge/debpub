package cmd

import (
	"context"
	"fmt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/spf13/cobra"

	"debpub/internal/debian"
	"debpub/internal/repo"
	"debpub/internal/storage"
)

var publishCmd = &cobra.Command{
	Use:   "publish [flags] <file.deb|pattern|directory...>",
	Short: "Publish one or more Debian packages to the repository",
	Long: `Publishes Debian (.deb, .udeb, .ddeb) packages to the target repository using the 4-phase staged upload protocol:
1. Upload package binaries to pool/
2. Generate and upload index files (Packages, Packages.gz, Packages.bz2, Packages.xz)
3. Activate release manifest (Release, Release.gpg, InRelease)
4. Release distributed lock

Positional arguments can be:
- One or more individual .deb file paths:
    debpub publish -c stable ./pkg1.deb ./pkg2.deb
- Wildcard/glob patterns (evaluated directly or shell-expanded):
    debpub publish -c stable "./dist/*.deb"
- Directories (scanned recursively for .deb, .udeb, and .ddeb packages):
    debpub publish -c stable ./build-output/`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := loadConfigWithPrecedence(cmd); err != nil {
			return fmt.Errorf("config error: %w", err)
		}

		if cfg.Codename == "" {
			return fmt.Errorf("missing required --codename (-c) or codename in config file")
		}

		debFiles, err := debian.CollectDebFiles(args)
		if err != nil {
			return fmt.Errorf("package collection error: %w", err)
		}

		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		backend, err := buildStorageBackend(ctx)
		if err != nil {
			return fmt.Errorf("storage backend error: %w", err)
		}

		var signer debian.Signer
		if cfg.Sign {
			signer = debian.NewGPGSigner(cfg.GPGKey, cfg.GPGPassphrase, cfg.GPGExtraArgs)
		}

		publisher := repo.NewPublisher(cfg, backend, signer)
		return publisher.PublishDebFiles(ctx, debFiles)
	},
}

func init() {
	rootCmd.AddCommand(publishCmd)
}

func buildStorageBackend(ctx context.Context) (storage.StorageBackend, error) {
	switch cfg.Storage {
	case "file":
		baseDir := cfg.LocalDir
		if baseDir == "" {
			baseDir = "."
		}
		return storage.NewFileBackend(baseDir)

	case "s3":
		if cfg.Bucket == "" {
			return nil, fmt.Errorf("missing required --bucket (-b) for s3 storage")
		}

		var optFns []func(*awsconfig.LoadOptions) error
		if cfg.S3Profile != "" {
			optFns = append(optFns, awsconfig.WithSharedConfigProfile(cfg.S3Profile))
		}
		if cfg.S3Region != "" {
			optFns = append(optFns, awsconfig.WithRegion(cfg.S3Region))
		}
		awsCfg, err := awsconfig.LoadDefaultConfig(ctx, optFns...)
		if err != nil {
			return nil, fmt.Errorf("failed loading AWS config: %w", err)
		}

		s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			if cfg.S3Endpoint != "" {
				o.BaseEndpoint = &cfg.S3Endpoint
			}
			if cfg.S3ForcePathStyle {
				o.UsePathStyle = true
			}
		})

		return storage.NewS3Backend(storage.S3Options{
			Client:        s3Client,
			Bucket:        cfg.Bucket,
			Prefix:        cfg.Prefix,
			LegacyLocking: cfg.S3LegacyLocking,
		}), nil

	case "sftp":
		if cfg.SFTPHost == "" {
			return nil, fmt.Errorf("missing required --sftp-host for sftp storage")
		}
		return storage.NewSFTPBackend(storage.SFTPOptions{
			Host:     cfg.SFTPHost,
			Port:     cfg.SFTPPort,
			User:     cfg.SFTPUser,
			Password: cfg.SFTPPassword,
			KeyPath:  cfg.SFTPKeyPath,
			BasePath: cfg.Prefix,
		})

	default:
		return nil, fmt.Errorf("unsupported storage backend: %q (supported: file, s3, sftp)", cfg.Storage)
	}
}
