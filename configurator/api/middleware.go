package api

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ─── CORS ─────────────────────────────────────────────────────────────────────

func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed := false
			for _, o := range allowedOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}
			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else if len(allowedOrigins) > 0 && allowedOrigins[0] != "*" {
				w.Header().Set("Access-Control-Allow-Origin", "http://"+allowedOrigins[0])
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key, X-Request-ID")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ─── Rate Limiter ─────────────────────────────────────────────────────────────

type rateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	rate     rate.Limit
	burst    int
}

func newRateLimiter(rps int) *rateLimiter {
	return &rateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(rps),
		burst:    rps,
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	lim, ok := rl.limiters[ip]
	if !ok {
		lim = rate.NewLimiter(rl.rate, rl.burst)
		rl.limiters[ip] = lim
	}
	rl.mu.Unlock()
	return lim.Allow()
}

func rateLimitMiddleware(rps int) func(http.Handler) http.Handler {
	if rps <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	rl := newRateLimiter(rps)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.allow(r.RemoteAddr) {
				jsonError(w, http.StatusTooManyRequests, "слишком много запросов")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ─── Request ID ───────────────────────────────────────────────────────────────

type reqIDKey struct{}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = fmt.Sprintf("%x", time.Now().UnixNano())
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), reqIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getReqID(r *http.Request) string {
	id, _ := r.Context().Value(reqIDKey{}).(string)
	return id
}

// ─── Recovery ─────────────────────────────────────────────────────────────────

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[PANIC] %s %s: %v", r.Method, r.URL.Path, rec)
				jsonError(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ─── CSRF Protection ───────────────────────────────────────────────────────────

// csrfMiddleware защищает state-changing методы (POST/PUT/DELETE/PATCH)
// от cross-site request forgery: если запрос пришёл из браузера с Origin,
// отличным от localhost, он отклоняется.
func csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET", "HEAD", "OPTIONS":
			next.ServeHTTP(w, r)
			return
		}
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = r.Header.Get("Referer")
		}
		if origin != "" && !isLocalhostOrigin(origin) {
			jsonError(w, http.StatusForbidden, "доступ запрещён: недопустимый Origin")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isLocalhostOrigin(origin string) bool {
	return strings.HasPrefix(origin, "http://localhost") ||
		strings.HasPrefix(origin, "http://127.0.0.1") ||
		strings.HasPrefix(origin, "http://[::1]")
}

// ─── Auth (API Key) ───────────────────────────────────────────────────────────

func authMiddleware(apiKey string) func(http.Handler) http.Handler {
	if apiKey == "" {
		log.Println("[WARN] API-ключ не задан — запросы не аутентифицируются")
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if subtle.ConstantTimeCompare(
				[]byte(r.Header.Get("X-API-Key")),
				[]byte(apiKey),
			) != 1 {
				jsonError(w, http.StatusUnauthorized, "неверный API-ключ")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
