package main

import (
	"context"
	"flag"
	"fmt"
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
	"salsgithub.com/site-audit/internal/slogx"
)

var (
	logger      = slogx.New(slog.LevelInfo)
	auditConfig = audit.Config{}
	local       bool
)

func logAndExit(message string, err error) {
	if logger == nil {
		fmt.Printf("%s error: %v\n", message, err)
	} else {
		logger.Error(message, "error", err)
	}
	os.Exit(1)
}

func main() {
	fs := flag.NewFlagSet("site-audit", flag.ContinueOnError)
	fs.BoolVar(&local, "local", false, "Running locally using .env in root")
	audit.AddFlags(auditConfig, fs)
	if err := fs.Parse(os.Args[1:]); err != nil {
		logAndExit("Error parsing flags", err)
	}
	if err := godotenv.Load(); err != nil && local {
		logAndExit("Error loading .env", err)
	}
	if err := envdecode.Decode(&auditConfig); err != nil {
		logAndExit("Error decoding audit config", err)
	}
	httpFetcher := fetcher.NewHTTPFetcher(auditConfig.Agent)
	linkExtractor := extractor.NewLinkExtractor(extractor.WithDefaultIgnores())
	auditor, err := audit.New(auditConfig, httpFetcher, linkExtractor)
	if err != nil {
		logAndExit("Error setting up auditer", err)
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
			logger.Error("Auditing complete with error", "err", err)
		} else {
			logger.Info("Auditing complete successfully")
		}
		return
	case s := <-sig:
		logger.Info("Signal received, shutting down", "signal", s)
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		select {
		case <-done:
			logger.Info("Graceful shutdown complete")
		case <-shutdownCtx.Done():
			logger.Info("Graceful shutdown timed out, force quitting")
		}
	}
}
