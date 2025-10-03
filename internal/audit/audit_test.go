package audit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/salsgithub/godst/graph"
	"github.com/stretchr/testify/require"
	"github.com/temoto/robotstxt"
	"salsgithub.com/site-audit/internal/extractor"
)

var (
	testConfig = Config{
		LogLevel:      "info",
		StartURL:      "https://example.com",
		Agent:         "agent",
		RespectRobots: true,
		MaxWorkers:    5,
		MaxDepth:      2,
		ValidSchemes:  "https,http",
	}
)

type mockFetcher struct {
	responses map[string]*http.Response
	err       error
}

func (m *mockFetcher) Fetch(ctx context.Context, u *url.URL) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	if response, ok := m.responses[u.String()]; ok {
		return response, nil
	}
	return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(""))}, nil
}

func buildResponse(body string, code int) *http.Response {
	return &http.Response{
		StatusCode: code,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func successResponse(body string) *http.Response {
	return buildResponse(body, http.StatusOK)
}

func notFoundResponse(body string) *http.Response {
	return buildResponse(body, http.StatusNotFound)
}

func forbiddenResponse(body string) *http.Response {
	return buildResponse(body, http.StatusForbidden)
}

type mockExtractor struct {
	values []string
	err    error
}

func (m *mockExtractor) Extract(u *url.URL, body io.Reader) ([]string, error) {
	return m.values, m.err
}

func TestAudit_New(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		fetcher   Fetcher
		extractor Extractor
		wantErr   error
	}{
		{
			name:    "Missing fetcher",
			wantErr: ErrNoFetcher,
		},
		{
			name:    "Missing extractor",
			fetcher: &mockFetcher{},
			wantErr: ErrNoExtractor,
		},
		{
			name: "Invalid start url",
			config: Config{
				StartURL: "https://a b.com",
			},
			fetcher:   &mockFetcher{},
			extractor: &mockExtractor{},
			wantErr:   ErrInvalidStartURL,
		},
		{
			name: "Invalid url scheme",
			config: Config{
				StartURL: "example.com",
			},
			fetcher:   &mockFetcher{},
			extractor: &mockExtractor{},
			wantErr:   ErrInvalidStartScheme,
		},
		{
			name: "Invalid max workers",
			config: Config{
				StartURL:   "https://example.com",
				MaxWorkers: -1,
			},
			fetcher:   &mockFetcher{},
			extractor: &mockExtractor{},
			wantErr:   ErrInvalidMaxWorkers,
		},
		{
			name: "Invalid max depth",
			config: Config{
				StartURL:   "https://example.com",
				MaxWorkers: 5,
				MaxDepth:   -1,
			},
			fetcher:   &mockFetcher{},
			extractor: &mockExtractor{},
			wantErr:   ErrInvalidMaxDepth,
		},
		{
			name: "Invalid log level",
			config: Config{
				LogLevel:   "something",
				StartURL:   "https://example.com",
				MaxWorkers: 5,
				MaxDepth:   2,
			},
			fetcher:   &mockFetcher{},
			extractor: &mockExtractor{},
			wantErr:   nil,
		},
		{
			name: "With valid schemes",
			config: Config{
				LogLevel:     "something",
				StartURL:     "https://example.com",
				MaxWorkers:   5,
				MaxDepth:     2,
				ValidSchemes: "https,http",
			},
			fetcher:   &mockFetcher{},
			extractor: &mockExtractor{},
			wantErr:   nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a, err := New(test.config, test.fetcher, test.extractor)
			if test.wantErr != nil {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.wantErr.Error())
			} else {
				require.NoError(t, err)
				require.NotNil(t, a)
			}
		})
	}
}

func TestAudit_Start(t *testing.T) {
	t.Run("respect robots returns error and stops start", func(t *testing.T) {
		mockFetcher := &mockFetcher{
			err: errors.New("network failure"),
		}
		mockExtractor := &mockExtractor{}
		a, err := New(testConfig, mockFetcher, mockExtractor)
		a.logger = slog.New(slog.DiscardHandler)
		require.NoError(t, err)
		require.NotNil(t, a)
		err = a.Start(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to respect robots")
	})
	t.Run("respect robots returns non 200 error and stops start", func(t *testing.T) {
		mockFetcher := &mockFetcher{
			responses: map[string]*http.Response{
				"https://example.com/robots.txt": forbiddenResponse("FORBIDDEN!"),
			},
		}
		mockExtractor := &mockExtractor{}
		a, err := New(testConfig, mockFetcher, mockExtractor)
		a.logger = slog.New(slog.DiscardHandler)
		require.NoError(t, err)
		require.NotNil(t, a)
		err = a.Start(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to respect robots")
	})
	t.Run("audit starts without respecting robots.txt", func(t *testing.T) {
		mockFetcher := &mockFetcher{
			responses: map[string]*http.Response{
				"https://example.com":        successResponse(`<html><body><a href="/page-a">A</a></body></html>`),
				"https://example.com/page-a": successResponse(`<html><body><a href="/page-b">B</a></body></html>`),
				"https://example.com/page-b": successResponse(`<html><body><h1>Page B</h1></body></html>`),
			},
		}
		mockExtractor := extractor.NewLinkExtractor(extractor.WithDefaultIgnores())
		c := testConfig
		c.RespectRobots = false
		a, err := New(c, mockFetcher, mockExtractor)
		require.NoError(t, err)
		require.NotNil(t, a)
		a.logger = slog.New(slog.DiscardHandler)
		err = a.Start(context.Background())
		require.NoError(t, err)
		require.Equal(t, 3, a.visited.Len())
	})
	t.Run("audit starts with querying robots.txt first", func(t *testing.T) {
		mockFetcher := &mockFetcher{
			responses: map[string]*http.Response{
				"https://example.com/robots.txt": successResponse(`User-agent: *\nDisallow:`),
				"https://example.com":            successResponse(`<html><body><a href="/page-a">A</a></body></html>`),
				"https://example.com/page-a":     successResponse(`<html><body><a href="/page-b">B</a></body></html>`),
				"https://example.com/page-b":     successResponse(`<html><body><h1>Page B</h1></body></html>`),
			},
		}
		mockExtractor := extractor.NewLinkExtractor(extractor.WithDefaultIgnores())
		c := testConfig
		a, err := New(c, mockFetcher, mockExtractor)
		require.NoError(t, err)
		require.NotNil(t, a)
		a.logger = slog.New(slog.DiscardHandler)
		err = a.Start(context.Background())
		require.NoError(t, err)
		require.Equal(t, 4, a.visited.Len())
		require.True(t, a.visited.Contains("https://example.com/robots.txt"))
	})
	t.Run("audit starts even if robots.txt was not found", func(t *testing.T) {
		mockFetcher := &mockFetcher{
			responses: map[string]*http.Response{
				"https://www.example.com/robots.txt": notFoundResponse(""),
				"https://example.com":                successResponse(`<html><body><a href="/page-a">A</a></body></html>`),
				"https://example.com/page-a":         successResponse(`<html><body><a href="/page-b">B</a></body></html>`),
				"https://example.com/page-b":         notFoundResponse(`<html><body><h1>Page B</h1></body></html>`),
			},
		}
		mockExtractor := extractor.NewLinkExtractor(extractor.WithDefaultIgnores())
		c := testConfig
		a, err := New(c, mockFetcher, mockExtractor)
		require.NoError(t, err)
		require.NotNil(t, a)
		a.logger = slog.New(slog.DiscardHandler)
		err = a.Start(context.Background())
		require.NoError(t, err)
		require.Equal(t, 4, a.visited.Len())
	})
}

func TestAudit_WorkerErrorDoesNotStopAuditing(t *testing.T) {
	t.Run("extract error", func(t *testing.T) {
		mockFetcher := &mockFetcher{
			responses: map[string]*http.Response{
				"https://example.com": successResponse(`invalid html`),
			},
		}
		mockExtractor := &mockExtractor{
			err: errors.New("extract error"),
		}
		c := testConfig
		c.RespectRobots = false
		a, err := New(c, mockFetcher, mockExtractor)
		require.NoError(t, err)
		require.NotNil(t, a)
		a.logger = slog.New(slog.DiscardHandler)
		err = a.Start(context.Background())
		require.NoError(t, err)
		require.Equal(t, a.visited.Len(), 1)
	})
	t.Run("fetch error", func(t *testing.T) {
		mockFetcher := &mockFetcher{
			err: errors.New("fetch error"),
		}
		mockExtractor := &mockExtractor{
			err: errors.New("extract error"),
		}
		c := testConfig
		c.RespectRobots = false
		a, err := New(c, mockFetcher, mockExtractor)
		require.NoError(t, err)
		require.NotNil(t, a)
		a.logger = slog.New(slog.DiscardHandler)
		err = a.Start(context.Background())
		require.NoError(t, err)
		require.Equal(t, a.visited.Len(), 1)
	})
	t.Run("fetch error 403", func(t *testing.T) {
		mockFetcher := &mockFetcher{
			responses: map[string]*http.Response{
				"http://example.com": forbiddenResponse("forbidden"),
			},
		}
		mockExtractor := &mockExtractor{
			err: errors.New("extract error"),
		}
		c := testConfig
		c.RespectRobots = false
		a, err := New(c, mockFetcher, mockExtractor)
		require.NoError(t, err)
		require.NotNil(t, a)
		a.logger = slog.New(slog.DiscardHandler)
		err = a.Start(context.Background())
		require.NoError(t, err)
		require.Equal(t, a.visited.Len(), 1)
	})
}

func TestAudit_ProcessLinks(t *testing.T) {
	newAudit := func() *Audit {
		mockFetcher := &mockFetcher{}
		mockExtractor := &mockExtractor{}
		c := testConfig
		c.RespectRobots = false
		a, err := New(c, mockFetcher, mockExtractor)
		require.NoError(t, err)
		require.NotNil(t, a)
		a.logger = slog.New(slog.DiscardHandler)
		return a
	}
	t.Run("skips already visited links", func(t *testing.T) {
		a := newAudit()
		a.logger = slog.New(slog.DiscardHandler)
		startURL, _ := url.Parse(testConfig.StartURL)
		startTask := &task{u: startURL, depth: 0}
		a.visited.Add(normaliseURL(startURL))
		initialLen := a.visited.Len()
		a.processLinks(startTask, []string{testConfig.StartURL})
		require.Equal(t, initialLen, a.visited.Len())
		require.True(t, a.tasks.IsEmpty())
	})
	t.Run("skips external links", func(t *testing.T) {
		a := newAudit()
		startURL, _ := url.Parse(testConfig.StartURL)
		startTask := &task{u: startURL, depth: 0}
		a.processLinks(startTask, []string{"http://somethingelse.com"})
		require.True(t, a.visited.IsEmpty())
		require.True(t, a.tasks.IsEmpty())
	})
	t.Run("skip links with disallowed scheme", func(t *testing.T) {
		a := newAudit()
		startURL, _ := url.Parse(testConfig.StartURL)
		startTask := &task{u: startURL, depth: 0}
		a.processLinks(startTask, []string{"mailto:test@example.com"})
		require.True(t, a.visited.IsEmpty())
		require.True(t, a.tasks.IsEmpty())
	})
	t.Run("skips links with url parse error", func(t *testing.T) {
		a := newAudit()
		startURL, _ := url.Parse(testConfig.StartURL)
		startTask := &task{u: startURL, depth: 0}
		a.processLinks(startTask, []string{"https://a b.com"})
		require.True(t, a.visited.IsEmpty())
		require.True(t, a.tasks.IsEmpty())
	})
	t.Run("skips links not allowed from robots.txt", func(t *testing.T) {
		a := newAudit()
		robotsBody := "User-Agent: *\nDisallow: /forbidden"
		robotsData, err := robotstxt.FromBytes([]byte(robotsBody))
		require.NoError(t, err)
		a.robotsData = robotsData
		startURL, _ := url.Parse(testConfig.StartURL)
		startTask := &task{u: startURL, depth: 0}
		a.processLinks(startTask, []string{fmt.Sprintf("%v/forbidden", testConfig.StartURL)})
		require.True(t, a.visited.IsEmpty())
		require.True(t, a.tasks.IsEmpty())
	})
}

func TestAudit_ExportGraph(t *testing.T) {
	mockFetcher := &mockFetcher{}
	mockExtractor := &mockExtractor{}
	c := testConfig
	c.RespectRobots = false
	a, err := New(c, mockFetcher, mockExtractor)
	require.NoError(t, err)
	require.NotNil(t, a)
	a.logger = slog.New(slog.DiscardHandler)
	a.siteGraph = graph.New[string]()
	a.siteGraph.AddEdge("https://example.com", "https://example.com/something", 1)
	t.Run("export without error", func(t *testing.T) {
		exported := false
		exportFunction := func(g *graph.Graph[string]) error {
			exported = true
			return nil
		}
		a.ExportGraph(exportFunction)
		require.True(t, exported)
	})
	t.Run("export with error", func(t *testing.T) {
		var exportErr error
		exportFunction := func(g *graph.Graph[string]) error {
			exportErr = errors.New("export error")
			return exportErr
		}
		a.ExportGraph(exportFunction)
		require.Error(t, exportErr)
	})
}
