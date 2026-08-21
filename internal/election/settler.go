package election

import (
	"context"
	"log/slog"
	"time"
)

// Settler periodically promotes election winners whose handover period has
// ended, so the change of syndic happens automatically without user action.
type Settler struct {
	service  *Service
	interval time.Duration
}

// NewSettler builds a settler that settles transitions every interval.
func NewSettler(store ElectionStore, interval time.Duration) *Settler {
	return &Settler{service: NewService(store), interval: interval}
}

// Run settles transitions every interval until ctx is cancelled.
func (s *Settler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.service.SettleTransitions(ctx); err != nil {
				slog.Error("settle transitions", "error", err)
			}
		}
	}
}
