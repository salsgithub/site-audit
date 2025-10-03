package audit

import "errors"

var (
	ErrInvalidStartURL    = errors.New("invalid start url")
	ErrInvalidStartScheme = errors.New("invalid start url scheme")
)

var (
	ErrInvalidMaxWorkers = errors.New("invaild max workers")
	ErrInvalidMaxDepth   = errors.New("invalid max depth")
)

var (
	ErrNoFetcher   = errors.New("no fetcher provided")
	ErrNoExtractor = errors.New("no extractor provided")
)
