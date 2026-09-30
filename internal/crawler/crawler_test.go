package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nikita-Mitskevich/crawler-cli/internal/fetcher"
)

func page(title string, links ...string) string {
	html := "<title>" + title + "</title>"
	for _, l := range links {
		html += `<a href="` + l + `">link</a>`
	}
	return html
}

type hitCounter struct {
	mu   sync.Mutex
	hits map[string]int
	next http.Handler
}

func (c *hitCounter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.mu.Lock()
	c.hits[r.URL.Path]++
	c.mu.Unlock()
	c.next.ServeHTTP(w, r)
}

func (c *hitCounter) count(path string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hits[path]
}

func sortTree(nodes []*Node) {
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Resource < nodes[j].Resource })
	for _, n := range nodes {
		sortTree(n.Links)
	}
}

func TestRun(t *testing.T) {
	html := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(body))
		}
	}

	mux := http.NewServeMux()
	mux.Handle("/{$}", html(page("Home", "/a", "/b", "/img.png", "/redirect", "/a#section", "https://example.org/x")))
	mux.Handle("/a", html(page("A", "/", "/a/deep")))
	mux.Handle("/a/deep", html(page("Deep", "/a/deep/deeper")))
	mux.Handle("/a/deep/deeper", html(page("Deeper")))
	mux.Handle("/b", html(page("B")))
	mux.HandleFunc("/img.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
	})
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/b", http.StatusFound)
	})

	tests := []struct {
		name          string
		depth         int
		wantTree      string
		wantRequested []string
		notRequested  []string
	}{
		{
			name:          "depth 0",
			depth:         0,
			wantTree:      "/\n",
			wantRequested: []string{"/"},
			notRequested:  []string{"/a", "/b"},
		},
		{
			name:          "depth 2",
			depth:         2,
			wantTree:      "/\n  /a\n    /a/deep\n  /b\n",
			wantRequested: []string{"/", "/a", "/b", "/img.png", "/redirect", "/a/deep"},
			notRequested:  []string{"/a/deep/deeper"},
		},
		{
			name:          "depth 1",
			depth:         1,
			wantTree:      "/\n  /a\n  /b\n",
			wantRequested: []string{"/", "/a", "/b", "/img.png", "/redirect"},
			notRequested:  []string{"/a/deep"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			counter := &hitCounter{hits: map[string]int{}, next: mux}
			srv := httptest.NewServer(counter)
			defer srv.Close()

			// Start with a trailing slash: href="/" resolves to exactly this string.
			opts := Options{URLs: []string{srv.URL + "/"}, MaxDepth: tc.depth, Workers: 3}
			roots := Run(context.Background(), opts, fetcher.NewHTTPFetcher(time.Second), discardLogger())

			sortTree(roots)
			if got := strings.ReplaceAll(render(roots, ""), srv.URL, ""); got != tc.wantTree {
				t.Errorf("tree:\ngot\n%swant\n%s", got, tc.wantTree)
			}
			for _, path := range tc.wantRequested {
				if n := counter.count(path); n != 1 {
					t.Errorf("%s requested %d times, want 1", path, n)
				}
			}
			for _, path := range tc.notRequested {
				if n := counter.count(path); n != 0 {
					t.Errorf("%s requested %d times, want 0", path, n)
				}
			}
		})
	}
}

func TestRunStopsOnTimeout(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(page("Home", "/slow")))
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(5 * time.Second):
		case <-r.Context().Done():
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	opts := Options{URLs: []string{srv.URL + "/"}, MaxDepth: 2, Workers: 3}
	roots := Run(ctx, opts, fetcher.NewHTTPFetcher(10*time.Second), discardLogger())

	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("Run took %v, want it to stop soon after the 300ms timeout", elapsed)
	}
	if got := strings.ReplaceAll(render(roots, ""), srv.URL, ""); got != "/\n" {
		t.Errorf("partial tree:\ngot\n%swant\n/\n", got)
	}
}

func TestRunLimitsConcurrency(t *testing.T) {
	const workers = 3
	var inFlight, maxInFlight atomic.Int32

	links := make([]string, 20)
	for i := range links {
		links[i] = fmt.Sprintf("/p/%d", i)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(page("Home", links...)))
	})
	mux.HandleFunc("/p/{n}", func(w http.ResponseWriter, r *http.Request) {
		cur := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			m := maxInFlight.Load()
			if cur <= m || maxInFlight.CompareAndSwap(m, cur) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		w.Header().Set("Content-Type", "text/html")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	opts := Options{URLs: []string{srv.URL + "/"}, MaxDepth: 1, Workers: workers}
	Run(context.Background(), opts, fetcher.NewHTTPFetcher(time.Second), discardLogger())

	got := maxInFlight.Load()
	if got > workers {
		t.Errorf("max concurrent requests: got %d, want at most %d", got, workers)
	}
	if got < 2 {
		t.Errorf("max concurrent requests: got %d, requests were not parallel", got)
	}
}
