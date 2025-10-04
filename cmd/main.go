package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joeshaw/envdecode"
	"github.com/joho/godotenv"
	"salsgithub.com/site-audit/internal/audit"
	"salsgithub.com/site-audit/internal/exporter"
	"salsgithub.com/site-audit/internal/extractor"
	"salsgithub.com/site-audit/internal/fetcher"
)

func main() {
	var (
		auditConfig audit.Config
		local       bool
	)
	fs := flag.NewFlagSet("site-audit", flag.ContinueOnError)
	fs.BoolVar(&local, "local", false, "Running locally using .env in root")
	audit.AddFlags(auditConfig, fs)
	if err := fs.Parse(os.Args[1:]); err != nil {
		slog.Error("Error parsing flags", "err", err)
		os.Exit(1)
	}
	if local {
		if err := godotenv.Load(); err != nil && local {
			slog.Error("Error loading .env", "err", err)
			os.Exit(1)
		}
	}
	if err := envdecode.Decode(&auditConfig); err != nil {
		slog.Error("Error loading .env", "err", err)
		os.Exit(1)
	}
	httpFetcher := fetcher.NewHTTPFetcher(auditConfig.Agent)
	linkExtractor := extractor.NewLinkExtractor(extractor.WithDefaultIgnores())
	auditor, err := audit.New(auditConfig, httpFetcher, linkExtractor)
	if err != nil {
		slog.Error("Auditor creation error", "err", err)
		os.Exit(1)
	}
	// Guarantee export of graph regardless of how auditor exits
	defer func() {
		graphVizExporter := exporter.NewGraphVizExporter("./out")
		auditor.ExportGraph(graphVizExporter.Export)
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	done := make(chan error, 1)
	go func() {
		done <- auditor.Start(ctx)
	}()
	select {
	case err := <-done:
		if err != nil {
			slog.Error("Auditing completed with error", "err", err)
		} else {
			slog.Info("Auditing complete successfully")
		}
		return
	case s := <-sig:
		slog.Info("Signal received, shutting down", "signal", s)
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		select {
		case <-done:
			slog.Info("Graceful shutdown complete")
		case <-shutdownCtx.Done():
			slog.Info("Graceful shutdown timed out, force quitting")
		}
	}
}
