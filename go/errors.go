package lobbytick

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Sentinels to test an *APIError with errors.Is.
var (
	ErrBadRequest  = errors.New("lobbytick: bad request")
	ErrNotFound    = errors.New("lobbytick: not found")
	ErrRateLimited = errors.New("lobbytick: rate limited")
)

// RateLimit holds a response's RateLimit-* headers; -1 when absent.
type RateLimit struct {
	// Limit is requests allowed per window (120 per minute today).
	Limit int
	// Remaining is requests left in the current window.
	Remaining int
	// Reset is how long until the window resets.
	Reset time.Duration
}

// APIError is an HTTP error answered by the API.
type APIError struct {
	StatusCode int
	// Message is the API's own "error" text, or "HTTP <status>" without one.
	Message string
	// Body is the raw response body.
	Body   []byte
	Header http.Header
	// RetryAfter is how long to wait before trying again: from Retry-After,
	// else RateLimit-Reset; 0 when the API gave no hint. Set on 429 (and on
	// 5xx when the API sends Retry-After).
	RetryAfter time.Duration
	RateLimit  RateLimit

	hinted bool // the API said how long to wait
}

func (e *APIError) Error() string {
	return fmt.Sprintf("lobbytick: %d: %s", e.StatusCode, e.Message)
}

// Is makes errors.Is(err, ErrNotFound) and friends work.
func (e *APIError) Is(target error) bool {
	switch target {
	case ErrBadRequest:
		return e.StatusCode == http.StatusBadRequest
	case ErrNotFound:
		return e.StatusCode == http.StatusNotFound
	case ErrRateLimited:
		return e.StatusCode == http.StatusTooManyRequests
	}
	return false
}

func intHeader(h http.Header, name string) int {
	n, err := strconv.Atoi(strings.TrimSpace(h.Get(name)))
	if err != nil {
		return -1
	}
	return n
}

func rateLimitOf(h http.Header) RateLimit {
	rl := RateLimit{Limit: intHeader(h, "RateLimit-Limit"), Remaining: intHeader(h, "RateLimit-Remaining"), Reset: -1}
	if n := intHeader(h, "RateLimit-Reset"); n >= 0 {
		rl.Reset = time.Duration(n) * time.Second
	}
	return rl
}

// ParseRetryAfter reads a Retry-After value, in seconds or as an HTTP date.
// ok is false when it is absent or invalid.
func ParseRetryAfter(raw string, now time.Time) (d time.Duration, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	if n, err := strconv.ParseFloat(raw, 64); err == nil {
		return max(0, time.Duration(n*float64(time.Second))), true
	}
	if t, err := http.ParseTime(raw); err == nil {
		return max(0, t.Sub(now).Round(time.Second)), true
	}
	return 0, false
}

func retryHint(h http.Header) (time.Duration, bool) {
	if d, ok := ParseRetryAfter(h.Get("Retry-After"), time.Now()); ok {
		return d, true
	}
	if n := intHeader(h, "RateLimit-Reset"); n >= 0 {
		return time.Duration(n) * time.Second, true
	}
	return 0, false
}
