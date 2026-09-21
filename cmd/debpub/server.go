package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"debpub/internal/api"
	"debpub/internal/config"
)

var serveCmd = &cobra.Command{
	Use:   "serve [flags]",
	Short: "Start HTTP/HTTPS server and Debian repository web browser",
	Long: `Starts an embedded HTTP or HTTPS server providing a REST API and a lightweight,
interactive web UI to inspect packages and metadata in the Debian repository.

Examples:
  # Start web browser on default port 8080
  debpub serve -c bookworm --dir ./my-deb-repo

  # Start web browser against remote AWS S3 repository
  debpub serve -b repo-deb.dev.example.com --s3-profile dev -c bookworm

  # Custom server port and bind address
  debpub serve --server-port 8088 --server-bind 0.0.0.0 --dir /var/repo

  # HTTPS with custom TLS certificates
  debpub serve --server-port 8443 --server-tls-cert /path/to/cert.pem --server-tls-key /path/to/key.pem`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := loadConfigWithPrecedence(cmd); err != nil {
			return fmt.Errorf("config error: %w", err)
		}

		normalizeAndLogConfig(cmd, cfg)

		ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		backend, err := buildStorageBackend(ctx)
		if err != nil {
			return fmt.Errorf("storage backend error: %w", err)
		}

		mgr := api.NewRepositoryManager(cfg, backend)

		// Pre-fetch index on launch (auto-discovering codename if omitted)
		slog.Info("Pre-fetching repository index", "codename", cfg.Codename, "component", cfg.Component)
		if err := mgr.SyncIndexes(ctx, cfg.Codename, cfg.Component); err != nil {
			slog.Warn("Could not pre-fetch index on startup (will be available via web UI fetch)", "err", err)
		}

		apiServer := api.NewServer(mgr)

		addr := net.JoinHostPort(cfg.ServerBind, strconv.Itoa(cfg.ServerPort))
		httpServer := &http.Server{
			Addr:              addr,
			Handler:           apiServer.Handler(),
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       60 * time.Second,
		}

		proto := "http"
		if cfg.ServerTLSCert != "" && cfg.ServerTLSKey != "" {
			proto = "https"
		}

		displayHost := cfg.ServerBind
		if displayHost == "" || displayHost == "0.0.0.0" {
			displayHost = "localhost"
		}
		uiURL := fmt.Sprintf("%s://%s:%d/", proto, displayHost, cfg.ServerPort)

		slog.Info("Starting Debian repository web server", "url", uiURL, "storage", cfg.Storage)

		serverErrCh := make(chan error, 1)
		go func() {
			if proto == "https" {
				serverErrCh <- httpServer.ListenAndServeTLS(cfg.ServerTLSCert, cfg.ServerTLSKey)
			} else {
				serverErrCh <- httpServer.ListenAndServe()
			}
		}()

		select {
		case err := <-serverErrCh:
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				return fmt.Errorf("server error: %w", err)
			}
			return nil

		case <-ctx.Done():
			slog.Info("Shutting down debpub web server...")
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			return httpServer.Shutdown(shutdownCtx)
		}
	},
}

func init() {
	serveCmd.Flags().IntVar(&cfg.ServerPort, "server-port", config.DefaultServerPort, "HTTP server listening port")
	serveCmd.Flags().StringVar(&cfg.ServerBind, "server-bind", config.DefaultServerBind, "HTTP server bind IP address or hostname")
	serveCmd.Flags().StringVar(&cfg.ServerTLSCert, "server-tls-cert", "", "Path to TLS certificate for HTTPS")
	serveCmd.Flags().StringVar(&cfg.ServerTLSKey, "server-tls-key", "", "Path to TLS private key for HTTPS")

	rootCmd.AddCommand(serveCmd)
}
