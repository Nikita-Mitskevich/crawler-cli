package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/Nikita-Mitskevich/crawler-cli/internal/config"
)

func main() {
	cfg := config.NewConfigMust(os.Args[1:])

	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05.000000")
	path := filepath.Join(
		cfg.LogFolder,
		fmt.Sprintf("%s.log", timestamp),
	)

	if err := os.MkdirAll(cfg.LogFolder, 0o755); err != nil {
		panic(fmt.Errorf("create log folder: %w", err))
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		panic(fmt.Errorf("open log file: %w", err))
	}
	defer file.Close()
	logger := slog.New(slog.NewTextHandler(file, nil))
}
