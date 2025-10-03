package fetcher

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHTTPFetcher_New(t *testing.T) {
	f := NewHTTPFetcher("agent")
	require.NotNil(t, f)
}

func TestHTTPFetcher_Fetch(t *testing.T) {
	t.Run("successful fetch", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ping"))
		}))
		defer server.Close()
		f := NewHTTPFetcher("agent")
		u, _ := url.Parse(server.URL)
		response, err := f.Fetch(t.Context(), u)
		require.NoError(t, err)
		defer response.Body.Close()
		require.Equal(t, response.StatusCode, http.StatusOK)
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, body, []byte("ping"))
	})
	t.Run("handle error from NewRequestWithContext", func(t *testing.T) {
		f := NewHTTPFetcher("agent")
		_, err := f.Fetch(t.Context(), &url.URL{
			Host: "a b.com",
		})
		require.Error(t, err)
	})
	t.Run("context cancellation stops fetching", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(1 * time.Second)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()
		f := NewHTTPFetcher("agent")
		u, _ := url.Parse(server.URL)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_, err := f.Fetch(ctx, u)
		require.Error(t, err)
		require.Contains(t, err.Error(), context.DeadlineExceeded.Error())
	})
}
