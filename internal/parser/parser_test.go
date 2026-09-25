package parser

import (
	"errors"
	"net/url"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
)

func TestParseHTML(t *testing.T) {
	base, err := url.Parse("https://site.com/a/b")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		html      string
		wantTitle string
		wantLinks []string
	}{
		{
			name:      "title and links",
			html:      `<html><head><title>  Hello  </title></head><body><a href="/about">x</a><a href="https://other.com/">y</a></body></html>`,
			wantTitle: "Hello",
			wantLinks: []string{"https://site.com/about", "https://other.com/"},
		},
		{
			name:      "no title",
			html:      `<a href="/x">x</a>`,
			wantTitle: "",
			wantLinks: []string{"https://site.com/x"},
		},
		{
			name:      "first title wins",
			html:      `<title>Page</title><svg><title>Icon</title></svg>`,
			wantTitle: "Page",
		},
		{
			name:      "relative links resolved against page URL",
			html:      `<a href="about"></a><a href="../x"></a>`,
			wantLinks: []string{"https://site.com/a/about", "https://site.com/x"},
		},
		{
			name: "non-http and empty hrefs skipped",
			html: `<a href="mailto:a@b.c"></a><a href="javascript:void(0)"></a><a href=""></a><a>no href</a>`,
		},
		{
			name:      "fragments removed",
			html:      `<a href="#top"></a><a href="/page#section"></a>`,
			wantLinks: []string{"https://site.com/a/b", "https://site.com/page"},
		},
		{
			name:      "uppercase tags and entities",
			html:      `<A HREF="/s?a=1&amp;b=2">x</A>`,
			wantLinks: []string{"https://site.com/s?a=1&b=2"},
		},
		{
			name:      "broken html",
			html:      `<title>T</title><div><a href="/x">`,
			wantTitle: "T",
			wantLinks: []string{"https://site.com/x"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			title, links, err := ParseHTML(strings.NewReader(tc.html), base)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if title != tc.wantTitle {
				t.Errorf("title: got %q, want %q", title, tc.wantTitle)
			}
			if got := toStrings(links); !slices.Equal(got, tc.wantLinks) {
				t.Errorf("links:\ngot  %q\nwant %q", got, tc.wantLinks)
			}
		})
	}
}

func TestParseHTMLReadError(t *testing.T) {
	base, _ := url.Parse("https://site.com/")
	errRead := errors.New("read failed")

	_, _, err := ParseHTML(iotest.ErrReader(errRead), base)
	if !errors.Is(err, errRead) {
		t.Errorf("got %v, want %v", err, errRead)
	}
}

func toStrings(links []*url.URL) []string {
	out := make([]string, 0, len(links))
	for _, l := range links {
		out = append(out, l.String())
	}
	return out
}
