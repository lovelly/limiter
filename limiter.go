package limiter

import (
	"context"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	*rate.Limiter
	n   int
	ctx context.Context
}

func NewRateLimit(limit int) *RateLimiter {
	r := &RateLimiter{Limiter: rate.NewLimiter(rate.Limit(limit), limit), n: limit, ctx: context.Background()}
	r.frirstLimit()
	return r
}

func (l *RateLimiter) frirstLimit() {
	for i := 0; i < l.n; i++ {
		l.Allow()
	}
}

func (l *RateLimiter) Go(f func()) {
	l.Wait(l.ctx)
	go f()
}

type ChanLimiter struct {
	queue chan struct{}
	n     int
}

func NewChanLimiter(limit int) *ChanLimiter {
	return &ChanLimiter{queue: make(chan struct{}, limit)}
}

func (l *ChanLimiter) Go(f func()) {
	l.queue <- struct{}{}
	go func() {
		f()
		<-l.queue
	}()
}
