package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxPageSize = 5 << 20

type Page struct {
	StatusCode int
	Body       []byte
}

type Fetcher interface {
	Fetch(ctx context.Context, rawURL string) (Page, error)
}

type HTTPFetcher struct {
	client         *http.Client
	requestTimeout time.Duration
}

func NewHTTPFetcher(requestTimeout time.Duration) *HTTPFetcher {
	return &HTTPFetcher{
		client: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
		},
		requestTimeout: requestTimeout,
	}
}

func (f *HTTPFetcher) Fetch(ctx context.Context, rawURL string) (Page, error) {
	reqCtx, cancel := context.WithTimeout(ctx, f.requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Page{}, fmt.Errorf("create http request: %w", err)
	}
	req.Header.Set("User-Agent", "nikita-crawler/0.1")
	resp, err := f.client.Do(req)
	if err != nil {
		return Page{}, fmt.Errorf("execute http request: %w", err)
	}

	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return Page{StatusCode: resp.StatusCode},
			fmt.Errorf("%w: status %d to %q", ErrRedirect, resp.StatusCode, resp.Header.Get("Location"))
	}
	if resp.StatusCode != http.StatusOK {
		return Page{StatusCode: resp.StatusCode}, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		return Page{StatusCode: resp.StatusCode}, fmt.Errorf("not html: %w", ErrNotHTML)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPageSize))
	if err != nil {
		return Page{StatusCode: resp.StatusCode}, fmt.Errorf("read body: %w", err)
	}
	return Page{StatusCode: resp.StatusCode, Body: body}, nil

}
