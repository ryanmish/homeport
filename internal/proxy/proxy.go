package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// Handler creates a reverse proxy handler for the given port
// It strips the /{port} prefix from the path and proxies to localhost:{port}
// WebSocket connections are handled automatically by httputil.ReverseProxy
func Handler(port int) http.Handler {
	// Use localhost instead of 127.0.0.1 to support both IPv4 and IPv6
	target, _ := url.Parse(fmt.Sprintf("http://localhost:%d", port))

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Customize the Director to strip the port prefix from the path
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// Strip /{port} prefix from path
		prefix := fmt.Sprintf("/%d", port)
		req.URL.Path = strings.TrimPrefix(req.URL.Path, prefix)
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}

		// Also update RawPath if set
		if req.URL.RawPath != "" {
			req.URL.RawPath = strings.TrimPrefix(req.URL.RawPath, prefix)
			if req.URL.RawPath == "" {
				req.URL.RawPath = "/"
			}
		}

		// Set X-Forwarded headers
		if clientIP := req.RemoteAddr; clientIP != "" {
			// RemoteAddr is "IP:port", extract just IP
			if idx := strings.LastIndex(clientIP, ":"); idx != -1 {
				clientIP = clientIP[:idx]
			}
			req.Header.Set("X-Forwarded-For", clientIP)
		}
		req.Header.Set("X-Forwarded-Host", req.Host)
		req.Header.Set("X-Forwarded-Proto", "http")

		// Important for WebSocket: preserve Host header
		req.Host = target.Host
	}

	// Custom error handler
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, fmt.Sprintf("Proxy error: %v", err), http.StatusBadGateway)
	}

	// Modify response to handle redirects correctly
	proxy.ModifyResponse = func(resp *http.Response) error {
		// If the backend sends a redirect, we need to rewrite the Location header
		// to include our port prefix
		if location := resp.Header.Get("Location"); location != "" {
			// Only rewrite relative redirects or redirects to localhost
			if strings.HasPrefix(location, "/") {
				resp.Header.Set("Location", fmt.Sprintf("/%d%s", port, location))
			}
		}
		return nil
	}

	return proxy
}

// HandlerWithBase creates a reverse proxy that strips a base path prefix.
// Used for services mounted at a sub-path (e.g., /code/* -> code-server)
func HandlerWithBase(port int, basePath string) http.Handler {
	target, _ := url.Parse(fmt.Sprintf("http://localhost:%d", port))

	proxy := httputil.NewSingleHostReverseProxy(target)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// Strip base path prefix
		req.URL.Path = strings.TrimPrefix(req.URL.Path, basePath)
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}

		// Also update RawPath if set
		if req.URL.RawPath != "" {
			req.URL.RawPath = strings.TrimPrefix(req.URL.RawPath, basePath)
			if req.URL.RawPath == "" {
				req.URL.RawPath = "/"
			}
		}

		// Set X-Forwarded headers
		if clientIP := req.RemoteAddr; clientIP != "" {
			if idx := strings.LastIndex(clientIP, ":"); idx != -1 {
				clientIP = clientIP[:idx]
			}
			req.Header.Set("X-Forwarded-For", clientIP)
		}
		req.Header.Set("X-Forwarded-Host", req.Host)
		req.Header.Set("X-Forwarded-Proto", "http")

		// Important for WebSocket: preserve Host header
		req.Host = target.Host
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, fmt.Sprintf("Proxy error: %v", err), http.StatusBadGateway)
	}

	// Modify response to rewrite redirects
	proxy.ModifyResponse = func(resp *http.Response) error {
		if location := resp.Header.Get("Location"); location != "" {
			if strings.HasPrefix(location, "/") {
				resp.Header.Set("Location", basePath+location)
			}
		}
		return nil
	}

	return proxy
}

// HandlerDirect creates a reverse proxy handler that does NOT strip any path prefix.
// Used for Referer-based routing where the request path should be forwarded as-is.
func HandlerDirect(port int) http.Handler {
	target, _ := url.Parse(fmt.Sprintf("http://localhost:%d", port))

	proxy := httputil.NewSingleHostReverseProxy(target)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// Set X-Forwarded headers
		if clientIP := req.RemoteAddr; clientIP != "" {
			if idx := strings.LastIndex(clientIP, ":"); idx != -1 {
				clientIP = clientIP[:idx]
			}
			req.Header.Set("X-Forwarded-For", clientIP)
		}
		req.Header.Set("X-Forwarded-Host", req.Host)
		req.Header.Set("X-Forwarded-Proto", "http")

		// Important for WebSocket: preserve Host header
		req.Host = target.Host
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, fmt.Sprintf("Proxy error: %v", err), http.StatusBadGateway)
	}

	return proxy
}

// DynamicHandler creates a handler that routes requests based on the port in the URL path
// Pattern: /{port}/... -> proxy to localhost:{port}/...
func DynamicHandler(allowedPorts func(port int) bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract port from path
		path := strings.TrimPrefix(r.URL.Path, "/")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) == 0 {
			http.NotFound(w, r)
			return
		}

		var port int
		if _, err := fmt.Sscanf(parts[0], "%d", &port); err != nil {
			http.NotFound(w, r)
			return
		}

		// Check if port is allowed
		if allowedPorts != nil && !allowedPorts(port) {
			http.Error(w, "Port not allowed", http.StatusForbidden)
			return
		}

		// Proxy the request
		Handler(port).ServeHTTP(w, r)
	})
}
