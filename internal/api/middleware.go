package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/gethomeport/homeport/internal/auth"
	"github.com/gethomeport/homeport/internal/share"
	"github.com/gethomeport/homeport/internal/store"
)

// portAuthMiddleware checks the sharing mode and handles authentication
func (s *Server) portAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		portStr := chi.URLParam(r, "port")
		port, err := strconv.Atoi(portStr)
		if err != nil {
			http.Error(w, "Invalid port", http.StatusBadRequest)
			return
		}

		// Check if port is in allowed range
		if port < s.cfg.PortRangeMin || port > s.cfg.PortRangeMax {
			http.Error(w, "Port out of range", http.StatusForbidden)
			return
		}

		// Get port info
		portInfo, err := s.store.GetPort(port)
		if err != nil {
			http.Error(w, "Port not found - is a dev server running?", http.StatusNotFound)
			return
		}

		// Check if share has expired (treat as private if expired)
		if portInfo.ExpiresAt != nil && time.Now().After(*portInfo.ExpiresAt) {
			// Share expired - reset to private
			_ = s.store.UpdatePortShare(port, "private", "", nil)
			portInfo.ShareMode = "private"
		}

		clientIP := getClientIP(r)
		userAgent := r.UserAgent()

		// Check sharing mode
		switch portInfo.ShareMode {
		case "public":
			// No auth required - log access and continue
			s.store.LogAccess(port, clientIP, userAgent, false)
			next.ServeHTTP(w, r)

		case "private":
			// Check for valid homeport session cookie (same as main app auth)
			cookie, err := r.Cookie(auth.SessionCookieName)
			if err == nil && s.auth.ValidateSession(cookie.Value) {
				s.store.LogAccess(port, clientIP, userAgent, true)
				next.ServeHTTP(w, r)
				return
			}
			// Not authenticated - redirect to login
			http.Redirect(w, r, "/login?next="+r.URL.Path, http.StatusFound)

		case "password":
			// Check for valid port-specific auth cookie
			if share.ValidateAuthCookie(r, port) {
				s.store.LogAccess(port, clientIP, userAgent, true)
				next.ServeHTTP(w, r)
				return
			}

			// Not authenticated - show password form or handle form submission
			s.handlePasswordAuth(w, r, port, portInfo)

		default:
			// Unknown mode, treat as private and check session
			cookie, err := r.Cookie(auth.SessionCookieName)
			if err == nil && s.auth.ValidateSession(cookie.Value) {
				s.store.LogAccess(port, clientIP, userAgent, true)
				next.ServeHTTP(w, r)
				return
			}
			http.Redirect(w, r, "/login?next="+r.URL.Path, http.StatusFound)
		}
	})
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For first (for proxied requests)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	// Check CF-Connecting-IP (Cloudflare)
	if cfIP := r.Header.Get("CF-Connecting-IP"); cfIP != "" {
		return cfIP
	}
	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// handlePasswordAuth shows the password form or validates the submitted password
func (s *Server) handlePasswordAuth(w http.ResponseWriter, r *http.Request, port int, portInfo *store.Port) {
	clientIP := getClientIP(r)

	// Check rate limiting
	if share.CheckRateLimit(clientIP) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(share.PasswordFormHTML(port, "Too many failed attempts. Please try again later.")))
		return
	}

	// Check if this is a form submission
	if r.Method == "POST" && r.URL.Path == "/"+strconv.Itoa(port)+"/_auth" {
		if err := r.ParseForm(); err != nil {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(share.PasswordFormHTML(port, "Invalid form submission")))
			return
		}

		password := r.FormValue("password")
		if share.VerifyPassword(password, portInfo.PasswordHash) {
			// Clear rate limiting on success
			share.ClearRateLimit(clientIP)

			// Set auth cookie (valid for 24 hours)
			share.SetAuthCookie(w, port, 24*time.Hour)

			// Redirect to the port root
			http.Redirect(w, r, "/"+strconv.Itoa(port)+"/", http.StatusSeeOther)
			return
		}

		// Wrong password - record failed attempt
		share.RecordFailedAttempt(clientIP)

		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(share.PasswordFormHTML(port, "Incorrect password")))
		return
	}

	// Show password form
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(share.PasswordFormHTML(port, "")))
}
