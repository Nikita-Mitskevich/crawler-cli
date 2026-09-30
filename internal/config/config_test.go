package config

import (
	"errors"
	"flag"
	"reflect"
	"testing"
	"time"
)

func validArgs(extra ...string) []string {
	args := []string{
		"--urls", "https://example.com",
		"--depth", "2",
		"--timeout", "1m",
		"--request-timeout", "10s",
		"--workers", "10",
	}
	return append(args, extra...)
}

func TestNewConfig(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want Config
	}{
		{
			name: "all flags",
			args: []string{
				"--urls", "https://google.com,https://example.com",
				"--depth", "3",
				"--timeout", "2m",
				"--request-timeout", "10s",
				"--output", "results",
				"--log", "logs",
				"--workers", "10",
			},
			want: Config{
				URLs:           []string{"https://google.com", "https://example.com"},
				Depth:          3,
				Timeout:        2 * time.Minute,
				RequestTimeout: 10 * time.Second,
				OutputFolder:   "results",
				LogFolder:      "logs",
				Workers:        10,
			},
		},
		{
			name: "defaults, spaces and empty elements",
			args: validArgs("--urls", " https://a.com ,, https://b.com,"),
			want: Config{
				URLs:           []string{"https://a.com", "https://b.com"},
				Depth:          2,
				Timeout:        time.Minute,
				RequestTimeout: 10 * time.Second,
				OutputFolder:   "./out/result",
				LogFolder:      "./out/logs",
				Workers:        10,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewConfig(tc.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got  %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

func TestNewConfigErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing urls", args: []string{"--depth", "2", "--timeout", "1m", "--request-timeout", "10s"}},
		{name: "only commas", args: validArgs("--urls", ",,")},
		{name: "invalid URL", args: validArgs("--urls", "example.com")},
		{name: "negative depth", args: validArgs("--depth", "-1")},
		{name: "missing request timeout", args: []string{"--urls", "https://example.com", "--depth", "2", "--timeout", "1m"}},
		{name: "invalid timeout", args: validArgs("--timeout", "abc")},
		{name: "unknown flag", args: validArgs("--errors", "5")},
		{name: "invalid workers count", args: validArgs("--workers", "12")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewConfig(tc.args); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestNewConfigHelp(t *testing.T) {
	_, err := NewConfig([]string{"-h"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Errorf("got %v, want flag.ErrHelp", err)
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      string
		wantEmpty bool
		wantErr   bool
	}{
		{name: "valid", input: "https://example.com/a?q=1", want: "https://example.com/a?q=1"},
		{name: "trimmed", input: "  https://example.com ", want: "https://example.com"},
		{name: "empty", input: "  ", wantEmpty: true},
		{name: "no scheme", input: "example.com", wantErr: true},
		{name: "ftp scheme", input: "ftp://example.com", wantErr: true},
		{name: "no host", input: "https://", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateURL(tc.input)

			switch {
			case tc.wantEmpty:
				if !errors.Is(err, ErrEmptyURL) {
					t.Errorf("got error %v, want ErrEmptyURL", err)
				}
			case tc.wantErr:
				if err == nil || errors.Is(err, ErrEmptyURL) {
					t.Errorf("got error %v, want invalid URL error", err)
				}
			default:
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tc.want {
					t.Errorf("got %q, want %q", got, tc.want)
				}
			}
		})
	}
}
