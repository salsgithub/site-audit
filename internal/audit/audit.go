package audit

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/salsgithub/godst/graph"
	"github.com/salsgithub/godst/queue"
	"github.com/salsgithub/godst/set"
	"github.com/temoto/robotstxt"
	"salsgithub.com/site-audit/internal/slogx"
)

type Fetcher interface {
	Fetch(ctx context.Context, u *url.URL) (*http.Response, error)
}

type Extractor interface {
	Extract(u *url.URL, body io.Reader) ([]string, error)
}

type task struct {
	u     *url.URL
	depth int
}

type Audit struct {
	config     Config
	logger     *slog.Logger
	fetcher    Fetcher
	extractor  Extractor
	startURL   *url.URL
	schemes    *set.Set[string]
	robotsData *robotstxt.RobotsData
	tasks      *queue.Queue[*task]
	visited    *set.Set[string]
	siteGraph  *graph.Graph[string]
	wg         sync.WaitGroup
	mu         sync.Mutex
}

func New(config Config, fetcher Fetcher, extractor Extractor) (*Audit, error) {
	if fetcher == nil {
		return nil, ErrNoFetcher
	}
	if extractor == nil {
		return nil, ErrNoExtractor
	}
	startURL, err := url.Parse(config.StartURL)
	if config.StartURL == "" || err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidStartURL, config.StartURL)
	}
	if startURL.Scheme == "" {
		return nil, fmt.Errorf("%w: %s", ErrInvalidStartScheme, config.StartURL)
	}
	if config.MaxWorkers < 0 {
		return nil, ErrInvalidMaxWorkers
	}
	if config.MaxDepth < 0 {
		return nil, ErrInvalidMaxDepth
	}
	logLevel := slog.LevelInfo
	if err := logLevel.UnmarshalText([]byte(config.LogLevel)); err != nil {
		fmt.Printf("Invalid log level %s, using info\n", config.LogLevel)
	}
	schemes := set.New("https")
	if config.ValidSchemes != "" {
		split := strings.Split(config.ValidSchemes, ",")
		schemes.Add(split...)
	}
	return &Audit{
		config:    config,
		logger:    slogx.New(logLevel),
		fetcher:   fetcher,
		extractor: extractor,
		startURL:  startURL,
		tasks:     queue.New[*task](),
		visited:   set.New[string](),
		siteGraph: graph.New[string](),
		schemes:   schemes,
	}, nil
}

func (a *Audit) Start(ctx context.Context) error {
	start := time.Now()
	if a.config.RespectRobots {
		if err := a.respectRobots(ctx); err != nil {
			return fmt.Errorf("failed to respect robots: %w", err)
		}
	}
	a.tasks.Enqueue(&task{
		u:     a.startURL,
		depth: 0,
	})
	a.visited.Add(a.startURL.String())
	for range a.config.MaxWorkers {
		a.wg.Add(1)
		go a.startWorker(ctx)
	}
	a.wg.Wait()
	a.logger.Info("Auditing finished", "duration_s", time.Since(start).Seconds(), "visited", a.visited.Len())
	return nil
}

func (a *Audit) ExportGraph(export func(g *graph.Graph[string]) error) {
	if err := export(a.siteGraph); err != nil {
		a.logger.Error("Error exporting site graph", "err", err)
	}
}

func (a *Audit) respectRobots(ctx context.Context) error {
	robotsURL := a.startURL.Scheme + "://" + a.startURL.Host + "/robots.txt"
	robots, err := url.Parse(robotsURL)
	if err != nil {
		return fmt.Errorf("error creating robots url: %w", err)
	}
	response, err := a.fetcher.Fetch(ctx, robots)
	if err != nil {
		return fmt.Errorf("error fetching robots.txt: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		a.logger.Info("robots.txt not found (404), proceeding to audit without restrictions")
		a.visited.Add(robotsURL)
		return nil
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("robots.txt not found or returned non 200/404 status: %d", response.StatusCode)
	}
	a.logger.Debug("robots.txt ok")
	b, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}
	robotsData, err := robotstxt.FromBytes(b)
	if err != nil {
		return fmt.Errorf("error parsing robots.txt data: %w", err)
	}
	a.logger.Debug("robots.txt configured")
	a.robotsData = robotsData
	a.visited.Add(robotsURL)
	return nil
}

func (a *Audit) startWorker(ctx context.Context) {
	defer a.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		default:

		}
		a.mu.Lock()
		if a.tasks.IsEmpty() {
			a.mu.Unlock()
			return
		}
		task, _ := a.tasks.Dequeue()
		a.mu.Unlock()
		a.logger.Debug("Fetching", "url", task.u.String())
		response, err := a.fetcher.Fetch(ctx, task.u)
		if err != nil {
			a.logger.Error("Failed to fetch url", "url", task.u.String(), "err", err)
			continue
		}
		defer response.Body.Close()
		if response.StatusCode >= http.StatusBadRequest {
			a.logger.Warn("Received non successful status code", "url", task.u.String(), "code", response.StatusCode)
			continue
		}
		links, err := a.extractor.Extract(task.u, response.Body)
		if err != nil {
			a.logger.Error("Error extracting links", "url", task.u.String(), "err", err)
			continue
		}
		a.logger.Debug("Links found", "links", links)
		a.processLinks(task, links)
	}
}

func (a *Audit) processLinks(t *task, links []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	baseURL := t.u
	baseHost := normaliseHost(baseURL.Host)
	for _, linkString := range links {
		parsedLink, err := url.Parse(linkString)
		if err != nil {
			a.logger.Debug("Malformed link", "link", linkString)
			continue
		}
		resolvedLink := baseURL.ResolveReference(parsedLink)
		resolvedHost := normaliseHost(resolvedLink.Host)
		if !a.schemes.Contains(resolvedLink.Scheme) {
			a.logger.Debug("Skipping link as scheme not permitted", "link", linkString, "scheme", resolvedLink.Scheme)
			continue
		}
		if baseHost != resolvedHost {
			a.logger.Debug("Skipping external link", "link", resolvedLink.String())
			continue
		}
		if a.robotsData != nil && !a.robotsData.TestAgent(resolvedLink.Path, a.config.Agent) {
			a.logger.Info("Skipping url disallowed by robots.txt", "url", resolvedLink.String())
			continue
		}
		canonicalURL := normaliseURL(resolvedLink)
		if a.visited.Contains(canonicalURL) {
			continue
		}
		a.visited.Add(canonicalURL)
		a.siteGraph.AddEdge(normaliseURL(baseURL), canonicalURL, 1)
		if t.depth+1 < a.config.MaxDepth {
			a.tasks.Enqueue(&task{
				u:     resolvedLink,
				depth: t.depth + 1,
			})
		}
	}
}

func normaliseHost(host string) string {
	return strings.TrimPrefix(host, "www.")
}

func normaliseURL(u *url.URL) string {
	path := u.Path
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}
	if path == "" {
		path = "/"
	}
	return u.Scheme + "://" + u.Host + path
}
