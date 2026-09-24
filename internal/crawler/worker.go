package crawler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/Nikita-Mitskevich/crawler-cli/internal/fetcher"
	"github.com/Nikita-Mitskevich/crawler-cli/internal/parser"
)

type Worker struct {
	fetcher  fetcher.Fetcher
	logger   *slog.Logger
	maxDepth int
}

func NewWorker(f fetcher.Fetcher, logger *slog.Logger, maxDepth int) *Worker {
	return &Worker{fetcher: f, logger: logger, maxDepth: maxDepth}
}

func (w *Worker) Run(ctx context.Context, tasks chan Task, results chan Result) {
	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-tasks:
			if !ok {
				return
			}

			result := w.process(ctx, task)

			select {
			case results <- result:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (w *Worker) process(ctx context.Context, task Task) Result {
	result := Result{Task: task}

	pageURL, err := url.Parse(task.URL)
	if err != nil {
		w.logger.Error("invalid page URL", "url", task.URL, "error", err)
		result.Err = fmt.Errorf("parse page URL: %w", err)
		return result
	}

	rootURL, err := url.Parse(task.Root)
	if err != nil {
		w.logger.Error("invalid root URL", "url", task.URL, "root", task.Root, "error", err)
		result.Err = fmt.Errorf("parse root URL: %w", err)
		return result
	}

	page, err := w.fetcher.Fetch(ctx, task.URL)
	if err != nil {
		switch {
		case ctx.Err() != nil:
			w.logger.Info("fetch aborted", "url", task.URL, "reason", ctx.Err())
		case errors.Is(err, fetcher.ErrRedirect):
			w.logger.Info("skipped: redirect", "url", task.URL, "status", page.StatusCode, "error", err)
		case errors.Is(err, fetcher.ErrNotHTML):
			w.logger.Info("skipped: not HTML", "url", task.URL, "status", page.StatusCode, "error", err)
		default:
			w.logger.Warn("fetch failed", "url", task.URL, "status", page.StatusCode, "error", err)
		}
		result.Err = err
		return result
	}
	w.logger.Info("page fetched", "url", task.URL, "status", page.StatusCode)

	title, links, err := parser.ParseHTML(bytes.NewReader(page.Body), pageURL)
	if err != nil {
		w.logger.Warn("parse HTML failed", "url", task.URL, "error", err)
		result.Err = fmt.Errorf("parse HTML: %w", err)
		return result
	}
	result.Title = title

	if task.Depth >= w.maxDepth {
		return result
	}

	result.Links = make([]string, 0, len(links))
	for _, link := range links {
		if link.Host != rootURL.Host {
			continue
		}
		result.Links = append(result.Links, link.String())
	}

	return result
}
