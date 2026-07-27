package notification

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/lib/pq"
)

type Listener struct {
	dsn     string
	hub     *Hub
	schemas []string
	tables  map[string][]string
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

func NewListener(dsn string, hub *Hub, schemas []string, tables map[string][]string) *Listener {
	return &Listener{
		dsn:     dsn,
		hub:     hub,
		schemas: schemas,
		tables:  tables,
		stopCh:  make(chan struct{}),
	}
}

func (l *Listener) Start(ctx context.Context) {
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		l.listen(ctx)
	}()
}

func (l *Listener) Stop() {
	close(l.stopCh)
	l.wg.Wait()
}

func (l *Listener) listen(ctx context.Context) {
	channelNames := l.buildChannels()

	listener := pq.NewListener(l.dsn, 10*time.Second, time.Minute, func(ev pq.ListenerEventType, err error) {
		if err != nil {
			slog.Warn("pq listener event", "event", ev, "error", err)
		}
	})
	defer listener.Close()

	for _, ch := range channelNames {
		if err := listener.Listen(ch); err != nil {
			slog.Error("failed to listen on channel", "channel", ch, "error", err)
			return
		}
	}

	slog.Info("notification listener started", "channels", len(channelNames))

	for {
		select {
		case <-ctx.Done():
			slog.Info("notification listener stopped")
			return
		case <-l.stopCh:
			slog.Info("notification listener stopped")
			return
		case n, ok := <-listener.Notify:
			if !ok {
				return
			}
			if n == nil {
				continue
			}
			l.hub.Broadcast(Event{
				Channel: n.Channel,
				Payload: n.Extra,
			})
		}
	}
}

func (l *Listener) buildChannels() []string {
	var channels []string
	seen := make(map[string]bool)

	for _, s := range l.schemas {
		for _, table := range l.tables[s] {
			ch := "rest_" + s + "_" + table
			if !seen[ch] {
				channels = append(channels, ch)
				seen[ch] = true
			}
		}
	}

	return channels
}
