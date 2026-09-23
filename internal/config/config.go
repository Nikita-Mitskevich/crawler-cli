package config

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type Config struct {
	URLs           []string
	Depth          int
	Timeout        time.Duration
	RequestTimeout time.Duration
	OutputPath     string
	LogPath        string
}

func NewConfig(args []string) (Config, error) {
	fs := flag.NewFlagSet("crawler-cli", flag.ContinueOnError)

	urls := fs.String("urls", "", "comma-separated list of start URLs")
	depth := fs.Int("depth", 0, "maximum crawl depth")
	timeout := fs.Duration("timeout", 0, "timeout for the whole crawl (e.g. 2m)")
	requestTimeout := fs.Duration("request-timeout", 0, "timeout for a single request (e.g. 10s)")
	output := fs.String("output", "./out/result.json", "path to the result JSON file")
	logPath := fs.String("log", "./out/crawler.log", "path to the log file")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	configURLs := strings.Split(*urls, ",")
	var validUrls []string
	for _, u := range configURLs {
		r, err := validateURL(u)
		if errors.Is(err, ErrEmptyURL) {
			continue
		}
		if err != nil {
			return Config{}, err
		}
		validUrls = append(validUrls, r)
	}

	if len(validUrls) == 0 {
		return Config{}, errors.New("at least one URL is required")
	}

	if *depth <= 0 {
		return Config{}, errors.New("crawl depth must be greater than 0")
	}

	if *timeout <= 0 {
		return Config{}, errors.New("timeout must be greater than 0")
	}

	if *requestTimeout <= 0 {
		return Config{}, errors.New("request timeout must be greater than 0")
	}

	if len(*output) == 0 {
		return Config{}, errors.New("output path is required")
	}

	if len(*logPath) == 0 {
		return Config{}, errors.New("log path is required")
	}

	return Config{
		URLs:           validUrls,
		Depth:          *depth,
		Timeout:        *timeout,
		RequestTimeout: *requestTimeout,
		OutputPath:     *output,
		LogPath:        *logPath,
	}, nil
}

func NewConfigMust(args []string) Config {
	config, err := NewConfig(args)
	if err != nil {
		panic(err)
	}
	return config
}

var ErrEmptyURL = errors.New("empty URL")

func validateURL(inputURL string) (string, error) {
	inputURL = strings.TrimSpace(inputURL)
	if inputURL == "" {
		return "", ErrEmptyURL
	}

	ref, err := url.Parse(inputURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	if (ref.Scheme != "http" && ref.Scheme != "https") || ref.Host == "" {
		return "", fmt.Errorf("invalid URL %q: scheme must be http or https and host must not be empty", inputURL)
	}

	return inputURL, nil
}
