package parser

import (
	"io"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

const limit = 5 << 20

func Parse(page io.Reader, base *url.URL) (string, []*url.URL, error) {
	var links []*url.URL
	var title string
	tokenizer := html.NewTokenizer(io.LimitReader(page, int64(limit)))
	for {
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {
			if tokenizer.Err() == io.EOF {
				return title, links, nil
			}
			return title, links, tokenizer.Err()
		}
		if tokenType != html.StartTagToken && tokenType != html.SelfClosingTagToken {
			continue
		}

		name, hasAttr := tokenizer.TagName()
		if string(name) == "title" {
			if tokenizer.Next() == html.TextToken && title == "" {
				title = strings.TrimSpace(string(tokenizer.Text()))
			}
			continue
		}
		if string(name) != "a" || !hasAttr {
			continue
		}

		for {
			key, val, more := tokenizer.TagAttr()
			if string(key) == "href" {
				if link := resolve(base, string(val)); link != nil {
					links = append(links, link)
				}
				break
			}
			if !more {
				break
			}
		}
	}
}

func resolve(base *url.URL, href string) *url.URL {
	href = strings.TrimSpace(href)
	if href == "" {
		return nil
	}
	ref, err := url.Parse(href)
	if err != nil {
		return nil
	}
	abs := base.ResolveReference(ref)
	if (abs.Scheme != "http" && abs.Scheme != "https") || abs.Host == "" {
		return nil
	}
	abs.Fragment = ""
	return abs
}
