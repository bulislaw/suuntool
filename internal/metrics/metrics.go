// Package metrics carries per-invocation request/cache counters without
// coupling the transport and cache packages to either CLI or MCP output.
package metrics

import (
	"context"
	"sync/atomic"
)

type contextKey struct{}

// Counters records work performed for one CLI command or one MCP tool call.
type Counters struct {
	serverRequests atomic.Int64
	cacheHits      atomic.Int64
}

// Snapshot is an immutable view suitable for rendering.
type Snapshot struct {
	ServerRequests int64
	CacheHits      int64
}

func New() *Counters { return &Counters{} }

func (c *Counters) RecordServerRequest() {
	if c != nil {
		c.serverRequests.Add(1)
	}
}

func (c *Counters) RecordCacheHit() {
	if c != nil {
		c.cacheHits.Add(1)
	}
}

func (c *Counters) Snapshot() Snapshot {
	if c == nil {
		return Snapshot{}
	}
	return Snapshot{ServerRequests: c.serverRequests.Load(), CacheHits: c.cacheHits.Load()}
}

// WithCounters associates c with ctx for API and cache work below it.
func WithCounters(ctx context.Context, c *Counters) context.Context {
	return context.WithValue(ctx, contextKey{}, c)
}

// FromContext returns the counters associated with ctx, if any.
func FromContext(ctx context.Context) *Counters {
	c, _ := ctx.Value(contextKey{}).(*Counters)
	return c
}
