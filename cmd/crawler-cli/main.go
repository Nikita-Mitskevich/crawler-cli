package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Nikita-Mitskevich/crawler-cli/internal/config"
	"github.com/Nikita-Mitskevich/crawler-cli/internal/crawler"
	"github.com/Nikita-Mitskevich/crawler-cli/internal/fetcher"
)

func main() {
	cfg, err := config.NewConfig(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}

	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(cfg config.Config) error {
	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05.000000")
	outputPath := filepath.Join(
		cfg.OutputFolder,
		fmt.Sprintf("%s.json", timestamp),
	)
	logPath := filepath.Join(
		cfg.LogFolder,
		fmt.Sprintf("%s.log", timestamp),
	)

	if err := os.MkdirAll(cfg.LogFolder, 0o755); err != nil {
		return fmt.Errorf("create log folder: %w", err)
	}
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer file.Close()
	logger := slog.New(slog.NewTextHandler(file, nil))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	opts := crawler.Options{
		URLs:     cfg.URLs,
		MaxDepth: cfg.Depth,
		Workers:  cfg.Workers,
	}

	httpFetcher := fetcher.NewHTTPFetcher(cfg.RequestTimeout)

	roots := crawler.Run(ctx, opts, httpFetcher, logger)

	if err := serializeAndWrite(roots, outputPath); err != nil {
		return err
	}

	return nil
}

func serializeAndWrite(roots []*crawler.Node, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output folder: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open output file: %w", err)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(roots); err != nil {
		return fmt.Errorf("encode json: %s: %w", path, err)
	}

	return nil
}
