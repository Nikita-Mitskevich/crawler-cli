package fetcher

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetch(t *testing.T) {
	var targetHits atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/page", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("<html>ok</html>"))
	})
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/target", http.StatusFound)
	})
	mux.HandleFunc("/target", func(w http.ResponseWriter, r *http.Request) {
		targetHits.Add(1)
		w.Header().Set("Content-Type", "text/html")
	})
	mux.HandleFunc("/image", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(time.Second):
		case <-r.Context().Done():
		}
	})
	mux.HandleFunc("/big", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(strings.Repeat("a", maxPageSize+1000)))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	f := NewHTTPFetcher(100 * time.Millisecond)

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantErr    error
		anyErr     bool
		wantLen    int
	}{
		{name: "html page", path: "/page", wantStatus: 200, wantLen: len("<html>ok</html>")},
		{name: "redirect", path: "/redirect", wantStatus: 302, wantErr: ErrRedirect},
		{name: "not found", path: "/missing", wantStatus: 404, anyErr: true},
		{name: "not html", path: "/image", wantStatus: 200, wantErr: ErrNotHTML},
		{name: "request timeout", path: "/slow", wantStatus: 0, wantErr: context.DeadlineExceeded},
		{name: "body is limited", path: "/big", wantStatus: 200, wantLen: maxPageSize},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			page, err := f.Fetch(context.Background(), srv.URL+tc.path)

			if page.StatusCode != tc.wantStatus {
				t.Errorf("status: got %d, want %d", page.StatusCode, tc.wantStatus)
			}

			switch {
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("error: got %v, want %v", err, tc.wantErr)
				}
			case tc.anyErr:
				if err == nil {
					t.Error("expected error, got nil")
				}
			default:
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(page.Body) != tc.wantLen {
					t.Errorf("body length: got %d, want %d", len(page.Body), tc.wantLen)
				}
			}
		})
	}

	if n := targetHits.Load(); n != 0 {
		t.Errorf("redirect was followed: /target requested %d times", n)
	}
}

func TestFetchCanceledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewHTTPFetcher(time.Second).Fetch(ctx, srv.URL)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("got %v, want context.Canceled", err)
	}
}
