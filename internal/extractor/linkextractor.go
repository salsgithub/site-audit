package extractor

import (
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/salsgithub/godst/set"
	"golang.org/x/net/html"
)

var defaultFileExtensions = []string{
	".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", // Images
	".zip", ".tar", ".gz", ".7z", ".rar", // Archives
	".mp3", ".mp4", ".mov", ".wav", ".flac", ".ogg", // Audio
	".doc", ".docx", ".pdf", ".txt", ".xls", ".xlsx", ".csv", ".ppt", // Docs
	".txt", ".rtf", // Text
	".exe", ".msi", ".deb", ".rpm", ".jar", ".sh", // Executables
	".css", ".js", // Web assets
	".ttf", ".otf", ".woff", ".woff2", // Fonts
}

var normaliseExtension = func(ext string) string {
	normalised := strings.ToLower(ext)
	if !strings.HasPrefix(normalised, ".") {
		return "." + normalised
	}
	return normalised
}

const (
	hyperTextReference string = "href"
	anchorTag          string = "a"
)

type Option func(*LinkExtractor)

type LinkExtractor struct {
	ignores *set.Set[string]
}

func NewLinkExtractor(options ...Option) *LinkExtractor {
	l := &LinkExtractor{ignores: set.New[string]()}
	for _, option := range options {
		option(l)
	}
	return l
}

func WithDefaultIgnores() Option {
	return WithIgnoredExtensions(defaultFileExtensions)
}

func WithIgnoredExtensions(extensions []string) Option {
	return func(l *LinkExtractor) {
		l.ignores.Clear()
		for _, ext := range extensions {
			l.ignores.Add(normaliseExtension(ext))
		}
	}
}

func WithAppendIgnoredExtensions(extensions []string) Option {
	return func(l *LinkExtractor) {
		for _, ext := range extensions {
			l.ignores.Add(normaliseExtension(ext))
		}
	}
}

func (l *LinkExtractor) Extract(u *url.URL, body io.Reader) ([]string, error) {
	links := set.New[string]()
	tokenizer := html.NewTokenizer(body)
	for {
		tokenType := tokenizer.Next()
		switch tokenType {
		case html.ErrorToken:
			err := tokenizer.Err()
			if err == io.EOF {
				return links.Values(), nil
			}
			return nil, err
		case html.StartTagToken, html.EndTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			if token.Data != anchorTag {
				continue
			}
			for _, attribute := range token.Attr {
				if attribute.Key != hyperTextReference {
					continue
				}
				fileExtension := strings.ToLower(path.Ext(attribute.Val))
				if fileExtension != "" && l.ignores.Contains(fileExtension) {
					continue
				}
				hrefURL, err := url.Parse(attribute.Val)
				if err != nil {
					continue
				}
				links.Add(u.ResolveReference(hrefURL).String())
			}
		}
	}
}
