package fetcher

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

type HTTPFetcher struct {
	client *http.Client
	agent  string
}

func NewHTTPFetcher(agent string) *HTTPFetcher {
	return &HTTPFetcher{
		client: &http.Client{Timeout: 5 * time.Second},
		agent:  agent,
	}
}

func (h *HTTPFetcher) Fetch(ctx context.Context, u *url.URL) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", h.agent)
	return h.client.Do(request)
}
