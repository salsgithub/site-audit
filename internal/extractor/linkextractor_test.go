package extractor

import (
	"bytes"
	"errors"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLinkExtractor_Extract(t *testing.T) {
	tests := []struct {
		name string
		html string
		want []string
	}{
		{
			name: "Invalid html",
			html: "no html",
			want: nil,
		},
		{
			name: "Ignore anchor without href",
			html: `<a>About</a>`,
			want: nil,
		},
		{
			name: "Ignore anchor tag with other attributes but no href",
			html: `<a id="top" class="link">No link<a>`,
			want: nil,
		},
		{
			name: "",
			html: `<a href="http://a b.com">Malformed</a>`,
			want: nil,
		},
		{
			name: "One loney link",
			html: `<a href="https://example.com/a">A</a>"`,
			want: []string{"https://example.com/a"},
		},
		{
			name: "One relative link",
			html: `<a href="/about">About</a>`,
			want: []string{"https://example.com/about"},
		},
		{
			name: "External and subdomain links",
			html: `<html><body><a href="https://other.com"></a><a href="https://sub.example.com"></a></body></html>`,
			want: []string{"https://other.com", "https://sub.example.com"},
		},
	}
	base, _ := url.Parse("https://example.com")
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := NewLinkExtractor(WithDefaultIgnores())
			reader := bytes.NewReader([]byte(test.html))
			links, err := e.Extract(base, reader)
			require.NoError(t, err)
			require.ElementsMatch(t, links, test.want)
		})
	}
}

func TestExtractor_WithAppendIgnoredExtensions(t *testing.T) {
	tests := []struct {
		name string
		html string
		want []string
	}{
		{
			name: "Ignore dat file",
			html: `<a href="/c.dat">About</a>`,
			want: nil,
		},
	}
	u, _ := url.Parse("https://example.com")
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := NewLinkExtractor(WithAppendIgnoredExtensions([]string{"dat"}))
			reader := bytes.NewReader([]byte(test.html))
			links, err := e.Extract(u, reader)
			require.NoError(t, err)
			require.ElementsMatch(t, links, test.want)
		})
	}
}

type errorReader struct{}

func (e *errorReader) Read(b []byte) (int, error) {
	return 0, errors.New("i/o error")
}

func TestExtractor_ErrorOnRead(t *testing.T) {
	u, _ := url.Parse("https://example.com")
	e := NewLinkExtractor()
	reader := &errorReader{}
	_, err := e.Extract(u, reader)
	require.Error(t, err)
}
