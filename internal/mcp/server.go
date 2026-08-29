package mcp

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bulislaw/suuntool/internal/api"
	"github.com/bulislaw/suuntool/internal/cache"
	"github.com/bulislaw/suuntool/internal/metrics"
	"github.com/bulislaw/suuntool/internal/session"
)

// Opts configures the MCP server. AllowWrite/AllowDestructive gate which
// tool tiers get registered. Transport defaults to StdioTransport when nil
// (production CLI). Tests pass an InMemoryTransport.
type Opts struct {
	AllowWrite       bool
	AllowDestructive bool
	BaseURL          string
	Timeout          time.Duration
	Transport        sdkmcp.Transport
	Verbose          bool
}

// Run starts the MCP server and blocks until the context is cancelled or the
// transport closes. Session is loaded lazily; if absent, authenticated tools
// surface AUTH_EXPIRED at call-time.
func Run(ctx context.Context, o Opts) error {
	timeout := o.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	sess, _ := session.Load() // may be nil — surfaced per-tool.
	sessionKey := ""
	if sess != nil {
		sessionKey = sess.SessionKey
	}
	cl := api.NewClient(o.BaseURL, sessionKey, timeout)
	tl := api.NewTimelineClient(sessionKey, timeout)
	store, err := cache.New(sess)
	if err != nil {
		store = nil // local caching is optional; requests must still work.
	}
	d := &deps{client: cl, timelineClient: tl, session: sess, cache: store, verbose: o.Verbose, logWriter: os.Stderr}

	s := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "suuntool", Version: "0"}, nil)
	registerVerboseMetrics(s, d)
	registerAll(s, d, o.AllowWrite, o.AllowDestructive)

	t := o.Transport
	if t == nil {
		t = &sdkmcp.StdioTransport{}
	}
	return s.Run(ctx, t)
}

// registerVerboseMetrics keeps MCP stdout protocol-only while giving each tool
// call an isolated counter set for -v diagnostics on stderr.
func registerVerboseMetrics(s *sdkmcp.Server, d *deps) {
	s.AddReceivingMiddleware(func(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
		return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
			call, ok := req.(*sdkmcp.CallToolRequest)
			if !ok {
				return next(ctx, method, req)
			}
			counts := metrics.New()
			result, err := next(metrics.WithCounters(ctx, counts), method, req)
			if d.verbose {
				writeMCPMetrics(d.logWriter, call.Params.Name, counts.Snapshot())
			}
			return result, err
		}
	})
}

func writeMCPMetrics(w io.Writer, tool string, counts metrics.Snapshot) {
	if w == nil {
		return
	}
	_, _ = fmt.Fprintf(w, "suuntool mcp %s: server requests=%d, cache hits=%d\n", tool, counts.ServerRequests, counts.CacheHits)
}

// registerAll wires the tier-gated registrars onto s. Exposed (unexported) so
// tests can build a server with deps they fully control.
func registerAll(s *sdkmcp.Server, d *deps, allowWrite, allowDestructive bool) {
	for _, r := range readRegistrars() {
		r(s, d)
	}
	if allowWrite {
		for _, r := range writeRegistrars() {
			r(s, d)
		}
	}
	if allowWrite && allowDestructive {
		for _, r := range destructiveRegistrars() {
			r(s, d)
		}
	}
}
