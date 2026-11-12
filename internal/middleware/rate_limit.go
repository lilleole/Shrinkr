package middleware

import (
	"encoding/json"
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type rateLimitEntry struct {
	requests  int
	resetTime time.Time
}

type RateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rateLimitEntry
	limit   int
	window  time.Duration
	counter uint64
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	if limit < 1 {
		panic("rate limit must be greater than zero")
	}
	if window <= 0 {
		panic("rate limit window must be greater than zero")
	}

	return &RateLimiter{
		entries: make(map[string]*rateLimitEntry),
		limit:   limit,
		window:  window,
	}
}

func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		now := time.Now()

		l.mu.Lock()

		entry, exists := l.entries[ip]

		if !exists || !now.Before(entry.resetTime) {
			entry = &rateLimitEntry{
				requests:  0,
				resetTime: now.Add(l.window),
			}
			l.entries[ip] = entry
		}

		entry.requests++
		allowed := entry.requests <= l.limit
		resetTime := entry.resetTime

		// Periodically remove expired entries to prevent unbounded growth.
		l.counter++
		if l.counter%256 == 0 {
			for key, value := range l.entries {
				if !now.Before(value.resetTime) {
					delete(l.entries, key)
				}
			}
		}

		l.mu.Unlock()

		if !allowed {
			retryAfter := int(math.Ceil(time.Until(resetTime).Seconds()))
			if retryAfter < 1 {
				retryAfter = 1
			}

			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)

			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "Too many requests. Please try again later.",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}

	return r.RemoteAddr
}
