package ratelimit

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/laurentpoirierfr/rest-trans/internal/config"
)

type LimiterManager struct {
	global *Limiter
	tables map[string]*Limiter
	cfg    config.RateLimitConfig
	mu     sync.RWMutex
}

func NewManager(cfg config.RateLimitConfig) *LimiterManager {
	m := &LimiterManager{
		tables: make(map[string]*Limiter),
		cfg:    cfg,
	}
	if cfg.Enabled {
		m.global = New(cfg.RPS, cfg.Burst)
		for table, tc := range cfg.PerTable {
			m.tables[table] = New(tc.RPS, tc.Burst)
		}
	}
	return m
}

func (m *LimiterManager) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.cfg.Enabled {
			c.Next()
			return
		}

		clientIP := c.ClientIP()
		tableName := c.Param("table")

		key := clientIP
		limiter := m.global

		if tableName != "" {
			m.mu.RLock()
			if tableLimiter, ok := m.tables[tableName]; ok {
				limiter = tableLimiter
				key = clientIP + ":" + tableName
			} else {
				key = clientIP
			}
			m.mu.RUnlock()
		}

		if !limiter.Allow(key) {
			c.Header("Retry-After", "1")
			c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", m.cfg.Burst))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    "PGRST429",
				"message": "Rate limit exceeded",
				"details": fmt.Sprintf("Try again in 1 second"),
			})
			return
		}

		c.Next()
	}
}

func (m *LimiterManager) Stop() {
	if m.global != nil {
		m.global.Stop()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, l := range m.tables {
		l.Stop()
	}
}
