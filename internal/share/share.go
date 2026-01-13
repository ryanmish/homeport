package share

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// cookieSecret is used to sign auth cookies
var cookieSecret []byte

// rateLimiter tracks failed password attempts per IP
var rateLimiter = struct {
	attempts map[string]int
	mu       sync.Mutex
}{attempts: make(map[string]int)}

func init() {
	// Load from environment or generate random secret
	if secret := os.Getenv("HOMEPORT_COOKIE_SECRET"); secret != "" {
		cookieSecret = []byte(secret)
	} else {
		// Generate random 32-byte secret for this session
		// Note: This means sessions are invalidated on restart
		// Set HOMEPORT_COOKIE_SECRET for persistent sessions
		cookieSecret = make([]byte, 32)
		if _, err := rand.Read(cookieSecret); err != nil {
			panic("failed to generate cookie secret: " + err.Error())
		}
	}

	// Clean up rate limiter every 15 minutes
	go func() {
		for {
			time.Sleep(15 * time.Minute)
			rateLimiter.mu.Lock()
			rateLimiter.attempts = make(map[string]int)
			rateLimiter.mu.Unlock()
		}
	}()
}

// CheckRateLimit returns true if the IP is rate limited (too many failed attempts)
func CheckRateLimit(ip string) bool {
	rateLimiter.mu.Lock()
	defer rateLimiter.mu.Unlock()
	return rateLimiter.attempts[ip] >= 5
}

// RecordFailedAttempt increments the failed attempt counter for an IP
func RecordFailedAttempt(ip string) {
	rateLimiter.mu.Lock()
	defer rateLimiter.mu.Unlock()
	rateLimiter.attempts[ip]++
}

// ClearRateLimit removes rate limiting for an IP after successful auth
func ClearRateLimit(ip string) {
	rateLimiter.mu.Lock()
	defer rateLimiter.mu.Unlock()
	delete(rateLimiter.attempts, ip)
}

// VerifyPassword checks if the provided password matches the hash
func VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// HashPassword creates a bcrypt hash of the password
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// SetAuthCookie sets a signed cookie for the given port
func SetAuthCookie(w http.ResponseWriter, port int, duration time.Duration) {
	expires := time.Now().Add(duration)
	value := signCookieValue(port, expires)

	http.SetCookie(w, &http.Cookie{
		Name:     fmt.Sprintf("homeport_auth_%d", port),
		Value:    value,
		Path:     "/", // Must be root for referer-based asset requests
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// ValidateAuthCookie checks if the request has a valid auth cookie for the port
func ValidateAuthCookie(r *http.Request, port int) bool {
	cookie, err := r.Cookie(fmt.Sprintf("homeport_auth_%d", port))
	if err != nil {
		return false
	}
	return verifyCookieValue(cookie.Value, port)
}

// signCookieValue creates a signed value: "port:expiry:signature"
func signCookieValue(port int, expires time.Time) string {
	data := fmt.Sprintf("%d:%d", port, expires.Unix())
	sig := computeHMAC(data)
	return base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", data, sig)))
}

// verifyCookieValue checks the signature and expiry
func verifyCookieValue(value string, expectedPort int) bool {
	decoded, err := base64.URLEncoding.DecodeString(value)
	if err != nil {
		return false
	}

	parts := strings.Split(string(decoded), ":")
	if len(parts) != 3 {
		return false
	}

	port, err := strconv.Atoi(parts[0])
	if err != nil || port != expectedPort {
		return false
	}

	expiry, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return false
	}

	if time.Now().Unix() > expiry {
		return false
	}

	// Verify signature
	data := fmt.Sprintf("%s:%s", parts[0], parts[1])
	expectedSig := computeHMAC(data)
	return parts[2] == expectedSig
}

func computeHMAC(data string) string {
	h := hmac.New(sha256.New, cookieSecret)
	h.Write([]byte(data))
	return base64.URLEncoding.EncodeToString(h.Sum(nil))
}

// PasswordFormHTML returns the HTML for the password form
func PasswordFormHTML(port int, errorMsg string) string {
	errorHTML := ""
	if errorMsg != "" {
		errorHTML = fmt.Sprintf(`<div class="error">%s</div>`, errorMsg)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Password Required - Port %d</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: #ffffff;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 20px;
        }
        .container {
            background: #ffffff;
            padding: 40px;
            border: 1px solid #e5e7eb;
            border-radius: 12px;
            width: 100%%;
            max-width: 400px;
        }
        .header {
            text-align: center;
            margin-bottom: 32px;
        }
        .port-badge {
            display: inline-flex;
            align-items: center;
            justify-content: center;
            width: 48px;
            height: 48px;
            background: #111827;
            color: white;
            border-radius: 12px;
            font-family: monospace;
            font-size: 14px;
            font-weight: 600;
            margin-bottom: 16px;
        }
        h1 {
            font-size: 24px;
            font-weight: 600;
            color: #111827;
            margin-bottom: 4px;
        }
        .subtitle {
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
        label {
            display: block;
            font-size: 14px;
            font-weight: 500;
            color: #374151;
            margin-bottom: 6px;
        }
        input[type="password"] {
            width: 100%%;
            padding: 12px 16px;
            border: 1px solid #e5e7eb;
            border-radius: 8px;
            font-size: 16px;
            margin-bottom: 16px;
            color: #111827;
        }
        input[type="password"]:focus {
            outline: none;
            border-color: #111827;
        }
        input[type="password"]::placeholder {
            color: #9ca3af;
        }
        button {
            width: 100%%;
            padding: 12px 16px;
            background: #111827;
            color: white;
            border: none;
            border-radius: 8px;
            font-size: 16px;
            font-weight: 500;
            cursor: pointer;
            transition: background 0.2s;
        }
        button:hover {
            background: #374151;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="port-badge">:%d</div>
            <h1>Password Required</h1>
            <p class="subtitle">This port is protected</p>
        </div>
        %s
        <form method="POST" action="/%d/_auth">
            <label for="password">Password</label>
            <input type="password" id="password" name="password" placeholder="Enter password" required autofocus>
            <button type="submit">Continue</button>
        </form>
    </div>
</body>
</html>`, port, port, errorHTML, port)
}
