package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// NewHandler creates a reverse proxy handler that forwards requests to baseURL
// while preserving the path after the configured prefix.
func NewHandler(prefix, baseURL string) (http.Handler, error) {
	target, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing base URL %q: %w", baseURL, err)
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			// Strip the route prefix from the incoming path
			inPath := r.In.URL.Path
			newPath := strings.TrimPrefix(inPath, prefix)
			if newPath == "" {
				newPath = "/"
			}
			if !strings.HasPrefix(newPath, "/") {
				newPath = "/" + newPath
			}

			// Build the outbound URL
			r.Out.URL.Scheme = target.Scheme
			r.Out.URL.Host = target.Host
			r.Out.URL.Path = joinPath(target.Path, newPath)
			r.Out.URL.RawQuery = r.In.URL.RawQuery

			// Set the Host header to match the target
			r.Out.Host = target.Host
		},
	}

	return proxy, nil
}

// joinPath joins two URL paths, avoiding double slashes.
func joinPath(base, rest string) string {
	if base == "" || base == "/" {
		return rest
	}
	base = strings.TrimSuffix(base, "/")
	if !strings.HasPrefix(rest, "/") {
		rest = "/" + rest
	}
	return base + rest
}
