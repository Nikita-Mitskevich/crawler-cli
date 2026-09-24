package fetcher

import "errors"

var (
	ErrRedirect = errors.New("redirect")
	ErrNotHTML  = errors.New("not html")
)
