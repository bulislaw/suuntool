package mcp

import (
	"context"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"io"

	"github.com/bulislaw/suuntool/internal/api"
	"github.com/bulislaw/suuntool/internal/cache"
	"github.com/bulislaw/suuntool/internal/session"
)

// tier is the gating bucket for a tool.
type tier int

const (
	tierRead tier = iota
	tierWrite
	tierDestructive
)

// deps holds runtime dependencies tool handlers close over.
// Populated by Run() in server.go.
type deps struct {
	client         *api.Client
	timelineClient *api.Client
	session        *session.Session
	cache          *cache.Store
	verbose        bool
	logWriter      io.Writer
}

// toolRegistrar registers one MCP tool on the given server, closing over d.
// Each tool has its own typed args struct, so registrars are not uniform —
// they wrap mcp.AddTool internally.
type toolRegistrar func(s *sdkmcp.Server, d *deps)

func cachedArtifact(ctx context.Context, d *deps, kind cache.Kind, id string, fetch func() (io.ReadCloser, error)) (io.ReadCloser, error) {
	return cache.Cached(ctx, d.cache, kind, id, fetch)
}

// invalidateWorkoutArtifacts is deliberately best effort: cache cleanup must
// not mask a successful server-side write from an MCP client.
func invalidateWorkoutArtifacts(d *deps, key string) {
	if d.cache == nil {
		return
	}
	_ = d.cache.Remove(cache.WorkoutSML, key)
	_ = d.cache.Remove(cache.WorkoutFIT, key)
}

func invalidateGuideArchive(d *deps, id string) {
	if d.cache != nil {
		_ = d.cache.Remove(cache.GuideArchive, id)
	}
}
