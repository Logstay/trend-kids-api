package http

import (
	"sync"
	"time"
)

// rateLimiter é uma janela fixa por chave (IP). Em memória: vale para uma instância só,
// o que basta para o painel de um único dono. Com várias instâncias, trocar por Redis.
type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	now     func() time.Time
	entries map[string]*window
}

type window struct {
	count   int
	resetAt time.Time
}

func newRateLimiter(limit int, w time.Duration, now func() time.Time) *rateLimiter {
	return &rateLimiter{limit: limit, window: w, now: now, entries: map[string]*window{}}
}

func (l *rateLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweep(now)

	e, ok := l.entries[key]
	if !ok || !now.Before(e.resetAt) {
		l.entries[key] = &window{count: 1, resetAt: now.Add(l.window)}
		return true
	}
	if e.count >= l.limit {
		return false
	}
	e.count++
	return true
}

// sweep remove janelas vencidas quando o mapa cresce, para não vazar memória com IPs únicos.
func (l *rateLimiter) sweep(now time.Time) {
	if len(l.entries) < 1024 {
		return
	}
	for k, e := range l.entries {
		if !now.Before(e.resetAt) {
			delete(l.entries, k)
		}
	}
}
