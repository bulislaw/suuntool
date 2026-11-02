package mcp

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net"
	"net/http"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// One server (and its account-scoped disk cache) is shared by all HTTP calls.
func newHTTPHandler(s *sdkmcp.Server, token string) http.Handler {
	mcpHandler := sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server {
		return s
	}, &sdkmcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	want := sha256.Sum256([]byte("Bearer " + token))
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.Handle("/mcp", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := sha256.Sum256([]byte(r.Header.Get("Authorization")))
		if token == "" || subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		mcpHandler.ServeHTTP(w, r)
	}))
	return http.NewCrossOriginProtection().Handler(mux)
}

func serveHTTP(ctx context.Context, addr string, handler http.Handler) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return serveHTTPListener(ctx, listener, handler)
}

func serveHTTPListener(ctx context.Context, listener net.Listener, handler http.Handler) error {
	srv := &http.Server{
		Handler: handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute,
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	done := make(chan struct{})
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if srv.Shutdown(shutdownCtx) != nil {
				_ = srv.Close()
			}
		case <-done:
		}
	}()
	err := srv.Serve(listener)
	close(done)
	// Serve returns as soon as Shutdown closes the listener, before handlers drain.
	<-shutdownDone
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
