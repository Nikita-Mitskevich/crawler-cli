package crawler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Nikita-Mitskevich/crawler-cli/internal/fetcher"
)

type fakeFetcher struct {
	pages map[string]string
	errs  map[string]error
}

func (f *fakeFetcher) Fetch(ctx context.Context, rawURL string) (fetcher.Page, error) {
	if err := ctx.Err(); err != nil {
		return fetcher.Page{}, err
	}
	if err, ok := f.errs[rawURL]; ok {
		return fetcher.Page{}, err
	}
	html, ok := f.pages[rawURL]
	if !ok {
		return fetcher.Page{StatusCode: 404}, errors.New("not found")
	}
	return fetcher.Page{StatusCode: 200, Body: []byte(html)}, nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func waitDone(t *testing.T, done chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("goroutine did not stop in time")
	}
}
