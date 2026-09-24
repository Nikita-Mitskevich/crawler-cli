package crawler

type Node struct {
	Resource string  `json:"resource"`
	Title    string  `json:"title"`
	Links    []*Node `json:"links"`
}

type Task struct {
	URL    string
	Root   string
	Depth  int
	Parent *Node
}

type Result struct {
	Task
	Title string
	Links []string
	Err   error
}
