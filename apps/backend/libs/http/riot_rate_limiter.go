package http

import (
	"io"
	nethttp "net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type riotRateWindow struct {
	limit    int
	duration time.Duration
	requests []time.Time
}

type riotRequestGate struct {
	mu           sync.Mutex
	windows      []*riotRateWindow
	blockedUntil time.Time
	concurrency  chan struct{}
}

type releaseOnCloseBody struct {
	io.ReadCloser
	releaseOnce sync.Once
	release     func()
}

func (b *releaseOnCloseBody) Close() error {
	err := b.ReadCloser.Close()
	b.releaseOnce.Do(b.release)
	return err
}

var (
	riotRequestGateOnce sync.Once
	globalRiotGate      *riotRequestGate
)

func positiveEnvInt(key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func getRiotRequestGate() *riotRequestGate {
	riotRequestGateOnce.Do(func() {
		globalRiotGate = &riotRequestGate{
			windows: []*riotRateWindow{
				{limit: positiveEnvInt("RIOT_API_SHORT_WINDOW_LIMIT", 18), duration: time.Second},
				{limit: positiveEnvInt("RIOT_API_LONG_WINDOW_LIMIT", 90), duration: 2 * time.Minute},
			},
			concurrency: make(chan struct{}, positiveEnvInt("RIOT_API_MAX_CONCURRENCY", 4)),
		}
	})
	return globalRiotGate
}

func (g *riotRequestGate) waitTurn() {
	for {
		g.mu.Lock()
		now := time.Now()
		next := g.blockedUntil

		for _, window := range g.windows {
			cutoff := now.Add(-window.duration)
			firstActive := 0
			for firstActive < len(window.requests) && !window.requests[firstActive].After(cutoff) {
				firstActive++
			}
			window.requests = window.requests[firstActive:]
			if len(window.requests) >= window.limit {
				availableAt := window.requests[len(window.requests)-window.limit].Add(window.duration)
				if availableAt.After(next) {
					next = availableAt
				}
			}
		}

		if !next.After(now) {
			for _, window := range g.windows {
				window.requests = append(window.requests, now)
			}
			g.mu.Unlock()
			return
		}

		g.mu.Unlock()
		time.Sleep(time.Until(next))
	}
}

func (g *riotRequestGate) blockFor(delay time.Duration) {
	if delay <= 0 {
		return
	}
	g.mu.Lock()
	blockedUntil := time.Now().Add(delay)
	if blockedUntil.After(g.blockedUntil) {
		g.blockedUntil = blockedUntil
	}
	g.mu.Unlock()
}

func isRiotAPIURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "api.riotgames.com" || strings.HasSuffix(host, ".api.riotgames.com")
}

func retryAfterDelay(response *nethttp.Response, attempt int) time.Duration {
	value := strings.TrimSpace(response.Header.Get("Retry-After"))
	if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds >= 0 {
		return time.Duration(seconds * float64(time.Second))
	}
	if retryAt, err := nethttp.ParseTime(value); err == nil {
		if delay := time.Until(retryAt); delay > 0 {
			return delay
		}
	}
	return time.Duration(1<<attempt) * time.Second
}

func getWithRiotPolicy(rawURL string) (*nethttp.Response, error) {
	if !isRiotAPIURL(rawURL) {
		return nethttp.Get(rawURL)
	}

	gate := getRiotRequestGate()
	const maxAttempts = 4
	for attempt := 0; attempt < maxAttempts; attempt++ {
		gate.concurrency <- struct{}{}
		gate.waitTurn()
		response, err := nethttp.Get(rawURL)
		if err != nil {
			<-gate.concurrency
			return nil, err
		}
		if response.StatusCode != nethttp.StatusTooManyRequests || attempt == maxAttempts-1 {
			response.Body = &releaseOnCloseBody{
				ReadCloser: response.Body,
				release:    func() { <-gate.concurrency },
			}
			return response, nil
		}

		delay := retryAfterDelay(response, attempt)
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		<-gate.concurrency
		gate.blockFor(delay)
	}

	return nil, nil
}
