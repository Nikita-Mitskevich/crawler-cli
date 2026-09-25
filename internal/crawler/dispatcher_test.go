package crawler

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"
)

func fakeCrawl(urls []string, site map[string][]string, failing map[string]bool) ([]*Node, map[string]int, map[string]int) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tasks := make(chan Task)
	results := make(chan Result)
	requests := map[string]int{}
	depths := map[string]int{}

	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		for task := range tasks {
			requests[task.URL]++
			depths[task.URL] = task.Depth

			res := Result{Task: task, Title: "title " + task.URL, Links: site[task.URL]}
			if failing[task.URL] {
				res.Err = errors.New("fetch failed")
			}
			select {
			case results <- res:
			case <-ctx.Done():
				return
			}
		}
	}()

	roots := NewDispatcher(urls).Run(ctx, tasks, results)
	<-workerDone
	return roots, requests, depths
}

func render(nodes []*Node, indent string) string {
	s := ""
	for _, n := range nodes {
		s += indent + n.Resource + "\n"
		s += render(n.Links, indent+"  ")
	}
	return s
}

func TestDispatcher(t *testing.T) {
	tests := []struct {
		name          string
		urls          []string
		site          map[string][]string
		failing       map[string]bool
		wantTree      string
		wantRequested []string
	}{
		{
			name:          "builds tree",
			urls:          []string{"A"},
			site:          map[string][]string{"A": {"B", "C"}, "B": {"D"}},
			wantTree:      "A\n  B\n    D\n  C\n",
			wantRequested: []string{"A", "B", "C", "D"},
		},
		{
			name:          "cycle",
			urls:          []string{"A"},
			site:          map[string][]string{"A": {"B"}, "B": {"A"}},
			wantTree:      "A\n  B\n",
			wantRequested: []string{"A", "B"},
		},
		{
			name:          "duplicate links requested once",
			urls:          []string{"A"},
			site:          map[string][]string{"A": {"B", "B", "C"}, "B": {"C"}},
			wantTree:      "A\n  B\n  C\n",
			wantRequested: []string{"A", "B", "C"},
		},
		{
			name:          "failed page is skipped with its links",
			urls:          []string{"A"},
			site:          map[string][]string{"A": {"B", "C"}, "B": {"D"}},
			failing:       map[string]bool{"B": true},
			wantTree:      "A\n  C\n",
			wantRequested: []string{"A", "B", "C"},
		},
		{
			name:          "two roots",
			urls:          []string{"A", "X"},
			site:          map[string][]string{"A": {"B"}, "X": {"Y"}},
			wantTree:      "A\n  B\nX\n  Y\n",
			wantRequested: []string{"A", "B", "X", "Y"},
		},
		{
			name:          "failed root gives empty result",
			urls:          []string{"A"},
			failing:       map[string]bool{"A": true},
			wantTree:      "",
			wantRequested: []string{"A"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			roots, requests, _ := fakeCrawl(tc.urls, tc.site, tc.failing)

			if roots == nil {
				t.Error("roots is nil, JSON would be null instead of []")
			}
			if got := render(roots, ""); got != tc.wantTree {
				t.Errorf("tree:\ngot\n%swant\n%s", got, tc.wantTree)
			}

			var requested []string
			for url, n := range requests {
				requested = append(requested, url)
				if n != 1 {
					t.Errorf("%s requested %d times, want 1", url, n)
				}
			}
			sort.Strings(requested)
			if strings.Join(requested, ",") != strings.Join(tc.wantRequested, ",") {
				t.Errorf("requested: got %v, want %v", requested, tc.wantRequested)
			}
		})
	}
}

func TestDispatcherDepthAndTitle(t *testing.T) {
	roots, _, depths := fakeCrawl([]string{"A"}, map[string][]string{"A": {"B"}, "B": {"C"}}, nil)

	for url, want := range map[string]int{"A": 0, "B": 1, "C": 2} {
		if depths[url] != want {
			t.Errorf("depth of %s: got %d, want %d", url, depths[url], want)
		}
	}
	if len(roots) != 1 || roots[0].Title != "title A" {
		t.Errorf("root title not set: %+v", roots)
	}
}

func TestDispatcherStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	tasks := make(chan Task)

	var roots []*Node
	done := make(chan struct{})
	go func() {
		roots = NewDispatcher([]string{"A"}).Run(ctx, tasks, make(chan Result))
		close(done)
	}()

	<-tasks
	cancel()
	waitDone(t, done)

	if _, ok := <-tasks; ok {
		t.Error("tasks channel is not closed")
	}
	if roots == nil {
		t.Error("roots is nil after cancel")
	}
}
