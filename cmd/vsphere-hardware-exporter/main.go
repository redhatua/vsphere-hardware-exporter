// Command vsphere-hardware-exporter exports VMware ESXi host hardware inventory as Prometheus metrics.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redhatua/vsphere-hardware-exporter/internal/cache"
	"github.com/redhatua/vsphere-hardware-exporter/internal/collector"
	"github.com/redhatua/vsphere-hardware-exporter/internal/config"
	"github.com/redhatua/vsphere-hardware-exporter/internal/inventory"
)

// Set at build time via -ldflags.
var (
	version = "dev"
	commit  = "none"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Parse(os.Args[1:], os.Getenv)
	if err != nil {
		return err
	}
	if cfg.ShowVersion {
		fmt.Printf("vsphere-hardware-exporter %s (%s)\n", version, commit)
		return nil
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))
	if cfg.Insecure {
		log.Warn("TLS certificate verification is disabled")
	}
	if cfg.ExportSerial {
		log.Info("serial number export enabled")
	}

	src := &inventory.Source{
		URL: cfg.URL, Username: cfg.Username, Password: cfg.Password,
		CAFile: cfg.CAFile, Insecure: cfg.Insecure,
		Include: cfg.Include, Exclude: cfg.Exclude, Logger: log,
	}
	store := cache.New(src, cfg.RefreshInterval, cfg.RefreshTimeout, log)
	reg := newRegistry(collector.New(store, collector.Options{ExportSerial: cfg.ExportSerial}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go store.Run(ctx)

	srv := &http.Server{Addr: cfg.ListenAddress, Handler: newHandler(reg, store), ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() {
		log.Info("listening", "address", cfg.ListenAddress, "version", version, "vcenter", cfg.URL.Hostname(), "refresh_interval", cfg.RefreshInterval)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
