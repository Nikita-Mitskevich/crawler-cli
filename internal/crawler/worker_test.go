package crawler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/Nikita-Mitskevich/crawler-cli/internal/fetcher"
)

func TestWorkerProcess(t *testing.T) {
	f := &fakeFetcher{
		pages: map[string]string{
			"https://site.com/": `<title>Home</title>
				<a href="/a">a</a>
				<a href="https://other.com/b">b</a>`,
		},
		errs: map[string]error{
			"https://site.com/broken": errors.New("connection refused"),
		},
	}
	w := NewWorker(f, discardLogger(), 2)
	parent := &Node{Resource: "https://site.com/"}

	tests := []struct {
		name      string
		task      Task
		wantTitle string
		wantLinks []string
		wantErr   bool
	}{
		{
			name:      "same-host links only",
			task:      Task{URL: "https://site.com/", Root: "https://site.com/", Depth: 0},
			wantTitle: "Home",
			wantLinks: []string{"https://site.com/a"},
		},
		{
			name:      "no links on max depth",
			task:      Task{URL: "https://site.com/", Root: "https://site.com/", Depth: 2, Parent: parent},
			wantTitle: "Home",
		},
		{
			name:    "fetch error",
			task:    Task{URL: "https://site.com/broken", Root: "https://site.com/", Depth: 1, Parent: parent},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := w.process(context.Background(), tc.task)

			if res.Task != tc.task {
				t.Errorf("task: got %+v, want %+v", res.Task, tc.task)
			}
			if (res.Err != nil) != tc.wantErr {
				t.Fatalf("error: got %v, want error: %v", res.Err, tc.wantErr)
			}
			if res.Title != tc.wantTitle {
				t.Errorf("title: got %q, want %q", res.Title, tc.wantTitle)
			}
			if !slices.Equal(res.Links, tc.wantLinks) {
				t.Errorf("links: got %q, want %q", res.Links, tc.wantLinks)
			}
		})
	}
}

func TestWorkerLogLevels(t *testing.T) {
	const pageURL = "https://site.com/x"

	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name    string
		ctx     context.Context
		err     error
		wantLog string
	}{
		{
			name:    "redirect",
			ctx:     context.Background(),
			err:     fmt.Errorf("%w: status 302", fetcher.ErrRedirect),
			wantLog: `level=INFO msg="skipped: redirect"`,
		},
		{
			name:    "not html",
			ctx:     context.Background(),
			err:     fmt.Errorf("%w: \"image/png\"", fetcher.ErrNotHTML),
			wantLog: `level=INFO msg="skipped: not HTML"`,
		},
		{
			name:    "real error",
			ctx:     context.Background(),
			err:     errors.New("connection refused"),
			wantLog: `level=WARN msg="fetch failed"`,
		},
		{
			name:    "program stopped",
			ctx:     canceled,
			wantLog: `level=INFO msg="fetch aborted"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			f := &fakeFetcher{errs: map[string]error{pageURL: tc.err}}
			w := NewWorker(f, slog.New(slog.NewTextHandler(&buf, nil)), 2)

			w.process(tc.ctx, Task{URL: pageURL, Root: pageURL})

			if !strings.Contains(buf.String(), tc.wantLog) {
				t.Errorf("log does not contain %s:\n%s", tc.wantLog, buf.String())
			}
		})
	}
}

func TestWorkerRun(t *testing.T) {
	f := &fakeFetcher{pages: map[string]string{"https://site.com/": "<title>Home</title>"}}
	task := Task{URL: "https://site.com/", Root: "https://site.com/"}

	start := func(ctx context.Context, tasks chan Task, results chan Result) chan struct{} {
		done := make(chan struct{})
		go func() {
			NewWorker(f, discardLogger(), 2).Run(ctx, tasks, results)
			close(done)
		}()
		return done
	}

	t.Run("one result per task, stops when tasks closed", func(t *testing.T) {
		tasks := make(chan Task)
		results := make(chan Result)
		done := start(context.Background(), tasks, results)

		tasks <- task
		if res := <-results; res.Title != "Home" {
			t.Errorf("title: got %q, want %q", res.Title, "Home")
		}
		close(tasks)
		waitDone(t, done)
	})

	t.Run("stops on cancel while nobody reads results", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		tasks := make(chan Task)
		done := start(ctx, tasks, make(chan Result))

		tasks <- task
		cancel()
		waitDone(t, done)
	})

	t.Run("stops on cancel while waiting for tasks", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		done := start(ctx, make(chan Task), make(chan Result))

		cancel()
		waitDone(t, done)
	})
}
