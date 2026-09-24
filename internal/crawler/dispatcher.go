package crawler

import (
	"context"
)

type Dispatcher struct {
	visited map[string]bool
	queue   []Task
	pending int
	storage []*Node
}

func NewDispatcher(URLs []string) *Dispatcher {
	tasks := make([]Task, 0, len(URLs))
	visited := map[string]bool{}
	for _, url := range URLs {
		tasks = append(tasks, Task{URL: url, Depth: 0, Root: url})
		visited[url] = true
	}
	return &Dispatcher{visited: visited, queue: tasks, pending: len(URLs), storage: []*Node{}}
}

func (d *Dispatcher) Run(ctx context.Context, tasks chan Task, results chan Result) []*Node {
	defer close(tasks)
	for {
		var out chan Task
		var task Task
		if len(d.queue) > 0 {
			task = d.queue[0]
			out = tasks
		}

		select {
		case r := <-results:
			d.pending--
			if r.Err != nil {
				break
			}
			resultNode := &Node{Title: r.Title, Resource: r.URL, Links: []*Node{}}
			if r.Parent == nil {
				d.storage = append(d.storage, resultNode)
			} else {
				r.Parent.Links = append(r.Parent.Links, resultNode)
			}
			if len(r.Links) == 0 {
				break
			}
			for _, t := range r.Links {
				if !d.visited[t] {
					d.queue = append(d.queue, Task{URL: t, Depth: r.Depth + 1, Parent: resultNode, Root: r.Root})
					d.pending++
					d.visited[t] = true
				}
			}
		case out <- task:
			d.queue = d.queue[1:]
		case <-ctx.Done():
			return d.storage
		}
		if d.pending == 0 {
			return d.storage
		}
	}
}
