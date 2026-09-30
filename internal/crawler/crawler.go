package crawler

import (
	"context"
	"log/slog"
	"sync"

	"github.com/Nikita-Mitskevich/crawler-cli/internal/fetcher"
)

type Options struct {
	URLs     []string
	MaxDepth int
	Workers  int
}

func Run(ctx context.Context, opts Options, f fetcher.Fetcher, logger *slog.Logger) []*Node {
	tasks := make(chan Task)
	results := make(chan Result)
	worker := NewWorker(f, logger, opts.MaxDepth)
	dispatcher := NewDispatcher(opts.URLs)
	var wg sync.WaitGroup
	for range opts.Workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker.Run(ctx, tasks, results)
		}()
	}
	roots := dispatcher.Run(ctx, tasks, results)
	wg.Wait()
	return roots
}
