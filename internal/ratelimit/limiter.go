package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Limiter struct {
	rate     rate.Limit
	burst    int
	limiters map[string]*keyLimiter
	mu       sync.Mutex
	stopCh   chan struct{}
}

type keyLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func New(rps float64, burst int) *Limiter {
	l := &Limiter{
		rate:     rate.Limit(rps),
		burst:    burst,
		limiters: make(map[string]*keyLimiter),
		stopCh:   make(chan struct{}),
	}
	go l.cleanup()
	return l
}

func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	kl, ok := l.limiters[key]
	if !ok {
		kl = &keyLimiter{
			limiter: rate.NewLimiter(l.rate, l.burst),
		}
		l.limiters[key] = kl
	}
	kl.lastSeen = time.Now()
	return kl.limiter.Allow()
}

func (l *Limiter) Stop() {
	close(l.stopCh)
}

func (l *Limiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopCh:
			return
		case <-ticker.C:
			l.mu.Lock()
			for key, kl := range l.limiters {
				if time.Since(kl.lastSeen) > 10*time.Minute {
					delete(l.limiters, key)
				}
			}
			l.mu.Unlock()
		}
	}
}

func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.limiters)
}
