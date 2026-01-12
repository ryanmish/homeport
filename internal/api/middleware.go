package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

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

		// Check sharing mode
		switch portInfo.ShareMode {
		case "public":
			// No auth required
			next.ServeHTTP(w, r)

		case "private":
			// Check for Cloudflare Access header
			cfEmail := r.Header.Get("CF-Access-Authenticated-User-Email")
			if cfEmail == "" && !s.cfg.DevMode {
				http.Error(w, "Unauthorized - Cloudflare Access required", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)

		case "password":
			// Check for valid auth cookie
			if share.ValidateAuthCookie(r, port) {
				next.ServeHTTP(w, r)
				return
			}

			// Not authenticated - show password form or handle form submission
			s.handlePasswordAuth(w, r, port, portInfo)

		default:
			// Unknown mode, treat as private
			next.ServeHTTP(w, r)
		}
	})
}

// handlePasswordAuth shows the password form or validates the submitted password
func (s *Server) handlePasswordAuth(w http.ResponseWriter, r *http.Request, port int, portInfo *store.Port) {
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
			// Set auth cookie (valid for 24 hours)
			share.SetAuthCookie(w, port, 24*time.Hour)

			// Redirect to the port root
			http.Redirect(w, r, "/"+strconv.Itoa(port)+"/", http.StatusSeeOther)
			return
		}

		// Wrong password
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
