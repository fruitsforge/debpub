package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"debpub/internal/config"
)

var (
	configFile string
	cfg        *config.Config

	rootCmd = &cobra.Command{
		Use:   "debpub",
		Short: "debpub is a modern, high-performance stateless Debian repository publishing tool",
		Long: `debpub is a stateless Debian repository management and publishing tool in Go.
It supports distributed locking on AWS S3 (and S3-compatible backends), SFTP,
and local filesystems, generating gzip, bzip2, and xz compressed indices,
and dual manifest formats (InRelease and Release.gpg).`,
	}
)

// Execute runs the debpub CLI command hierarchy.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	cfg = config.DefaultConfig()

	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "Path to debpub.json configuration file")
	rootCmd.PersistentFlags().StringVar(&cfg.Storage, "storage", "file", "Storage backend: s3, sftp, file")
	rootCmd.PersistentFlags().StringVarP(&cfg.Codename, "codename", "c", "", "Debian distribution codename (e.g. stable, bookworm, jammy)")
	rootCmd.PersistentFlags().StringVarP(&cfg.Component, "component", "m", "main", "Debian repository component (e.g. main, contrib, non-free)")
	rootCmd.PersistentFlags().StringVar(&cfg.LocalDir, "dir", "", "Local repository directory path when using file storage")

	// S3 Flags
	rootCmd.PersistentFlags().StringVarP(&cfg.Bucket, "bucket", "b", "", "AWS S3 bucket name")
	rootCmd.PersistentFlags().StringVar(&cfg.Prefix, "prefix", "", "S3 or remote directory path prefix")
	rootCmd.PersistentFlags().StringVar(&cfg.S3Endpoint, "s3-endpoint", "", "Custom S3 endpoint URL (for MinIO, Wasabi, Ceph, R2)")
	rootCmd.PersistentFlags().BoolVar(&cfg.S3ForcePathStyle, "s3-force-path-style", false, "Force path-style S3 URLs (e.g. for MinIO)")
	rootCmd.PersistentFlags().BoolVar(&cfg.S3LegacyLocking, "s3-legacy-locking", false, "Use optimistic check-then-put locking instead of S3 conditional writes")

	// SFTP Flags
	rootCmd.PersistentFlags().StringVar(&cfg.SFTPHost, "sftp-host", "", "SFTP server host")
	rootCmd.PersistentFlags().IntVar(&cfg.SFTPPort, "sftp-port", 22, "SFTP server port")
	rootCmd.PersistentFlags().StringVar(&cfg.SFTPUser, "sftp-user", "", "SFTP username")
	rootCmd.PersistentFlags().StringVar(&cfg.SFTPPassword, "sftp-password", "", "SFTP password")
	rootCmd.PersistentFlags().StringVar(&cfg.SFTPKeyPath, "sftp-key", "", "SFTP SSH private key file path")

	// Lock Flags
	rootCmd.PersistentFlags().BoolVar(&cfg.LockEnabled, "lock", true, "Enable distributed repository locking")
	rootCmd.PersistentFlags().DurationVar(&cfg.LockTimeout, "lock-timeout", cfg.LockTimeout, "Maximum duration to wait for repository lock")
	rootCmd.PersistentFlags().DurationVar(&cfg.LockTTL, "lock-ttl", cfg.LockTTL, "Lock expiration TTL before being marked stale")

	// Repository Metadata Flags
	rootCmd.PersistentFlags().StringVar(&cfg.Origin, "origin", "", "Debian repository Origin header")
	rootCmd.PersistentFlags().StringVar(&cfg.Label, "label", "", "Debian repository Label header")
	rootCmd.PersistentFlags().StringVar(&cfg.Suite, "suite", "", "Debian repository Suite header")
	rootCmd.PersistentFlags().StringVar(&cfg.Description, "description", "", "Debian repository Description")
	rootCmd.PersistentFlags().BoolVar(&cfg.PreserveVersions, "preserve-versions", false, "Keep older versions of packages in index")

	// Signing Flags
	rootCmd.PersistentFlags().BoolVarP(&cfg.Sign, "sign", "s", false, "GPG sign Release manifest (generates Release.gpg and InRelease)")
	rootCmd.PersistentFlags().StringVarP(&cfg.GPGKey, "gpg-key", "k", "", "GPG signing key ID or email")
	rootCmd.PersistentFlags().StringVar(&cfg.GPGPassphrase, "gpg-passphrase", "", "GPG key passphrase")
}

// loadConfigWithPrecedence merges --config debpub.json into cfg without overwriting CLI flags explicitly set by user.
func loadConfigWithPrecedence(cmd *cobra.Command) error {
	if configFile == "" {
		return nil
	}

	fileCfg := config.DefaultConfig()
	if err := config.LoadConfigFile(configFile, fileCfg); err != nil {
		return err
	}

	// Apply file configuration only for flags that were NOT explicitly set on the command line
	applyStringIfNotChanged(cmd, "storage", &cfg.Storage, fileCfg.Storage)
	applyStringIfNotChanged(cmd, "codename", &cfg.Codename, fileCfg.Codename)
	applyStringIfNotChanged(cmd, "component", &cfg.Component, fileCfg.Component)
	applyStringIfNotChanged(cmd, "dir", &cfg.LocalDir, fileCfg.LocalDir)
	applyStringIfNotChanged(cmd, "bucket", &cfg.Bucket, fileCfg.Bucket)
	applyStringIfNotChanged(cmd, "prefix", &cfg.Prefix, fileCfg.Prefix)
	applyStringIfNotChanged(cmd, "s3-endpoint", &cfg.S3Endpoint, fileCfg.S3Endpoint)
	applyBoolIfNotChanged(cmd, "s3-force-path-style", &cfg.S3ForcePathStyle, fileCfg.S3ForcePathStyle)
	applyBoolIfNotChanged(cmd, "s3-legacy-locking", &cfg.S3LegacyLocking, fileCfg.S3LegacyLocking)
	applyStringIfNotChanged(cmd, "sftp-host", &cfg.SFTPHost, fileCfg.SFTPHost)
	applyIntIfNotChanged(cmd, "sftp-port", &cfg.SFTPPort, fileCfg.SFTPPort)
	applyStringIfNotChanged(cmd, "sftp-user", &cfg.SFTPUser, fileCfg.SFTPUser)
	applyStringIfNotChanged(cmd, "sftp-password", &cfg.SFTPPassword, fileCfg.SFTPPassword)
	applyStringIfNotChanged(cmd, "sftp-key", &cfg.SFTPKeyPath, fileCfg.SFTPKeyPath)
	applyBoolIfNotChanged(cmd, "lock", &cfg.LockEnabled, fileCfg.LockEnabled)
	applyDurationIfNotChanged(cmd, "lock-timeout", &cfg.LockTimeout, fileCfg.LockTimeout)
	applyDurationIfNotChanged(cmd, "lock-ttl", &cfg.LockTTL, fileCfg.LockTTL)
	applyStringIfNotChanged(cmd, "origin", &cfg.Origin, fileCfg.Origin)
	applyStringIfNotChanged(cmd, "label", &cfg.Label, fileCfg.Label)
	applyStringIfNotChanged(cmd, "suite", &cfg.Suite, fileCfg.Suite)
	applyStringIfNotChanged(cmd, "description", &cfg.Description, fileCfg.Description)
	applyBoolIfNotChanged(cmd, "preserve-versions", &cfg.PreserveVersions, fileCfg.PreserveVersions)
	applyBoolIfNotChanged(cmd, "sign", &cfg.Sign, fileCfg.Sign)
	applyStringIfNotChanged(cmd, "gpg-key", &cfg.GPGKey, fileCfg.GPGKey)
	applyStringIfNotChanged(cmd, "gpg-passphrase", &cfg.GPGPassphrase, fileCfg.GPGPassphrase)

	for k, v := range fileCfg.ExtraMetadata {
		cfg.ExtraMetadata[k] = v
	}

	return nil
}

func applyStringIfNotChanged(cmd *cobra.Command, flagName string, target *string, fileVal string) {
	if fileVal != "" && !cmd.Flags().Changed(flagName) {
		*target = fileVal
	}
}

func applyBoolIfNotChanged(cmd *cobra.Command, flagName string, target *bool, fileVal bool) {
	if !cmd.Flags().Changed(flagName) {
		*target = fileVal
	}
}

func applyIntIfNotChanged(cmd *cobra.Command, flagName string, target *int, fileVal int) {
	if fileVal != 0 && !cmd.Flags().Changed(flagName) {
		*target = fileVal
	}
}

func applyDurationIfNotChanged(cmd *cobra.Command, flagName string, target *time.Duration, fileVal time.Duration) {
	if fileVal > 0 && !cmd.Flags().Changed(flagName) {
		*target = fileVal
	}
}
