package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	SessionCookieName = "homeport_session"
	SessionDuration   = 30 * 24 * time.Hour // 30 days
)

// Auth handles authentication for Homeport
type Auth struct {
	passwordHash []byte
	cookieSecret []byte

	// Rate limiting
	mu           sync.Mutex
	failedLogins map[string][]time.Time
}

// New creates a new Auth instance
func New(passwordHash, cookieSecret string) *Auth {
	return &Auth{
		passwordHash: []byte(passwordHash),
		cookieSecret: []byte(cookieSecret),
		failedLogins: make(map[string][]time.Time),
	}
}

// IsConfigured returns true if a password has been set
func (a *Auth) IsConfigured() bool {
	return len(a.passwordHash) > 0
}

// Session represents a user session
type Session struct {
	CreatedAt int64 `json:"c"`
	ExpiresAt int64 `json:"e"`
}

// CheckPassword verifies the password and returns true if correct
func (a *Auth) CheckPassword(password string) bool {
	if !a.IsConfigured() {
		return true // No auth configured, allow access
	}
	err := bcrypt.CompareHashAndPassword(a.passwordHash, []byte(password))
	return err == nil
}

// SetPasswordHash updates the password hash at runtime
func (a *Auth) SetPasswordHash(hash []byte) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.passwordHash = hash
}

// IsRateLimited checks if the IP has too many failed login attempts
func (a *Auth) IsRateLimited(ip string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Clean up old entries
	cutoff := time.Now().Add(-15 * time.Minute)
	attempts := a.failedLogins[ip]
	valid := make([]time.Time, 0)
	for _, t := range attempts {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	a.failedLogins[ip] = valid

	// 5 attempts per 15 minutes
	return len(valid) >= 5
}

// RecordFailedLogin records a failed login attempt
func (a *Auth) RecordFailedLogin(ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.failedLogins[ip] = append(a.failedLogins[ip], time.Now())
}

// CreateSession creates a new session cookie value
func (a *Auth) CreateSession() (string, error) {
	session := Session{
		CreatedAt: time.Now().Unix(),
		ExpiresAt: time.Now().Add(SessionDuration).Unix(),
	}

	data, err := json.Marshal(session)
	if err != nil {
		return "", err
	}

	// Sign the session data
	sig := a.sign(data)
	return base64.URLEncoding.EncodeToString(data) + "." + base64.URLEncoding.EncodeToString(sig), nil
}

// ValidateSession checks if a session cookie is valid
func (a *Auth) ValidateSession(cookie string) bool {
	parts := strings.Split(cookie, ".")
	if len(parts) != 2 {
		return false
	}

	data, err := base64.URLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}

	sig, err := base64.URLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}

	// Verify signature
	expectedSig := a.sign(data)
	if !hmac.Equal(sig, expectedSig) {
		return false
	}

	// Parse and check expiration
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return false
	}

	return time.Now().Unix() < session.ExpiresAt
}

func (a *Auth) sign(data []byte) []byte {
	h := hmac.New(sha256.New, a.cookieSecret)
	h.Write(data)
	return h.Sum(nil)
}

// SetSessionCookie sets the session cookie on the response
func (a *Auth) SetSessionCookie(w http.ResponseWriter, r *http.Request) error {
	session, err := a.CreateSession()
	if err != nil {
		return err
	}

	// Determine if we should use Secure flag (behind HTTPS)
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    session,
		Path:     "/",
		MaxAge:   int(SessionDuration.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	return nil
}

// ClearSessionCookie clears the session cookie
func (a *Auth) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}

// GetClientIP extracts the client IP from the request
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For first (for proxied requests)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	// Fall back to RemoteAddr
	return strings.Split(r.RemoteAddr, ":")[0]
}

// Middleware returns an HTTP middleware that requires authentication
func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If no password configured, allow all access
		if !a.IsConfigured() {
			next.ServeHTTP(w, r)
			return
		}

		// Allow localhost API requests without auth (for CLI inside container)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			host := r.Host
			if colonIdx := strings.LastIndex(host, ":"); colonIdx != -1 {
				host = host[:colonIdx]
			}
			if host == "localhost" || host == "127.0.0.1" || host == "::1" {
				// Check if request is coming directly to localhost (not proxied)
				if r.Header.Get("X-Forwarded-For") == "" && r.Header.Get("X-Real-IP") == "" {
					next.ServeHTTP(w, r)
					return
				}
			}
		}

		// Check for valid session cookie
		cookie, err := r.Cookie(SessionCookieName)
		if err == nil && a.ValidateSession(cookie.Value) {
			// Sliding expiration: refresh session if more than 1 day old
			if a.shouldRefreshSession(cookie.Value) {
				a.SetSessionCookie(w, r)
			}
			next.ServeHTTP(w, r)
			return
		}

		// Not authenticated - redirect to login
		// For API requests, return 401
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Redirect to login page
		http.Redirect(w, r, "/login", http.StatusFound)
	})
}

// shouldRefreshSession returns true if the session should be refreshed (sliding expiration)
func (a *Auth) shouldRefreshSession(cookie string) bool {
	parts := strings.Split(cookie, ".")
	if len(parts) != 2 {
		return false
	}

	data, err := base64.URLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return false
	}

	// Refresh if session was created more than 1 day ago
	return time.Now().Unix()-session.CreatedAt > 86400
}

// LoginPage returns the HTML for the login page
func LoginPage(error string) string {
	errorHTML := ""
	if error != "" {
		errorHTML = fmt.Sprintf(`<div class="error">%s</div>`, error)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Login - Homeport</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: #ffffff;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            color: #111827;
        }
        .container {
            background: #ffffff;
            border: 1px solid #e5e7eb;
            border-radius: 12px;
            padding: 40px;
            width: 100%%;
            max-width: 400px;
        }
        .logo {
            text-align: center;
            margin-bottom: 32px;
        }
        .logo-box {
            width: 48px;
            height: 48px;
            background: #111827;
            border-radius: 12px;
            display: flex;
            align-items: center;
            justify-content: center;
            margin: 0 auto 16px;
        }
        .logo-box svg {
            width: 28px;
            height: 28px;
            color: white;
        }
        .logo h1 {
            font-size: 24px;
            font-weight: 600;
            margin-bottom: 4px;
            color: #111827;
        }
        .logo p {
            color: #6b7280;
            font-size: 14px;
        }
        .error {
            background: #fef2f2;
            border: 1px solid #fecaca;
            color: #dc2626;
            padding: 12px 16px;
            border-radius: 8px;
            margin-bottom: 20px;
            font-size: 14px;
        }
        form { display: flex; flex-direction: column; gap: 16px; }
        label {
            font-size: 14px;
            font-weight: 500;
            color: #374151;
        }
        input[type="password"] {
            width: 100%%;
            padding: 12px 16px;
            font-size: 16px;
            border: 1px solid #e5e7eb;
            border-radius: 8px;
            background: #ffffff;
            color: #111827;
            margin-top: 6px;
            outline: none;
            transition: border-color 0.2s;
        }
        input[type="password"]:focus {
            border-color: #111827;
        }
        input[type="password"]::placeholder {
            color: #9ca3af;
        }
        button {
            width: 100%%;
            padding: 12px 16px;
            font-size: 16px;
            font-weight: 500;
            background: #111827;
            color: #fff;
            border: none;
            border-radius: 8px;
            cursor: pointer;
            transition: background 0.2s;
            margin-top: 8px;
        }
        button:hover { background: #374151; }
        button:disabled {
            background: #9ca3af;
            cursor: not-allowed;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="logo">
            <div class="logo-box">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>
                    <polyline points="9 22 9 12 15 12 15 22"/>
                </svg>
            </div>
            <h1>Homeport</h1>
        </div>
        %s
        <form method="POST" action="/login">
            <div>
                <label for="password">Password</label>
                <input type="password" id="password" name="password" placeholder="Enter your password" required autofocus>
            </div>
            <button type="submit">Sign In</button>
        </form>
    </div>
</body>
</html>`, errorHTML)
}
