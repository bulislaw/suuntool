package api

import (
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net"

	"github.com/bulislaw/suuntool/internal/metrics"
)

// multiCloser wraps a primary reader and closes both the inner reader (e.g. gzip)
// and the underlying response body on Close.
type multiCloser struct {
	io.Reader
	closers []io.Closer
}

func (m *multiCloser) Close() error {
	var last error
	// Close in reverse order: inner reader first, then outer body.
	for i := len(m.closers) - 1; i >= 0; i-- {
		if err := m.closers[i].Close(); err != nil {
			last = err
		}
	}
	return last
}

// DoStream executes the request and returns a streaming ReadCloser for 2xx responses.
// The caller MUST close the returned ReadCloser.
//
// Transparent gzip decompression:
//   - If the response carries Content-Encoding: gzip, the body is gunzipped via that header.
//   - Otherwise the first two bytes are peeked; if they are the gzip magic (0x1f 0x8b), the
//     body is still gunzipped (Suunto's 247 service gzips unconditionally without setting the
//     header).
//   - Otherwise the raw body is returned.
//
// Non-2xx responses are mapped to *Error exactly as Do does.
func (c *Client) DoStream(ctx context.Context, method, path string, body io.Reader, headers map[string]string) (io.ReadCloser, error) {
	req, err := c.newRequest(ctx, method, path, body, headers)
	if err != nil {
		return nil, err
	}

	metrics.FromContext(ctx).RecordServerRequest()
	resp, err := c.HTTP.Do(req)
	if err != nil {
		var nerr net.Error
		if errors.As(err, &nerr) && nerr.Timeout() {
			return nil, &Error{Code: "NETWORK", Message: "timeout: " + err.Error(), Exit: 3}
		}
		return nil, &Error{Code: "NETWORK", Message: err.Error(), Exit: 3}
	}

	if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
		// Read up to 4 KB for the error message, then close.
		limited := io.LimitReader(resp.Body, 4096)
		b, _ := io.ReadAll(limited)
		resp.Body.Close()
		switch {
		case resp.StatusCode == 401:
			return nil, &Error{Code: "AUTH_EXPIRED", Message: "session rejected", Hint: "Run: suuntool login", HTTP: 401, Exit: 4}
		case resp.StatusCode == 403:
			return nil, &Error{Code: "FORBIDDEN", Message: snippet(b), HTTP: 403, Exit: 7}
		case resp.StatusCode == 404:
			return nil, &Error{Code: "NOT_FOUND", Message: snippet(b), HTTP: 404, Exit: 6}
		case resp.StatusCode >= 500:
			return nil, &Error{Code: "SERVER", Message: snippet(b), HTTP: resp.StatusCode, Exit: 5}
		default:
			return nil, &Error{Code: "SERVER", Message: snippet(b), HTTP: resp.StatusCode, Exit: 5}
		}
	}

	// 2xx — decide whether to gunzip.
	if resp.Header.Get("Content-Encoding") == "gzip" {
		// Header-declared gzip.
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
			return nil, &Error{Code: "SERVER", Message: "gzip init failed: " + err.Error(), Exit: 5}
		}
		return &multiCloser{
			Reader:  gr,
			closers: []io.Closer{gr, resp.Body},
		}, nil
	}

	// Peek the first two bytes to detect gzip magic without consuming the stream.
	buf := bufio.NewReader(resp.Body)
	magic, err := buf.Peek(2)
	if err == nil && len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		// Magic-bytes gzip (no header set).
		gr, err := gzip.NewReader(buf)
		if err != nil {
			resp.Body.Close()
			return nil, &Error{Code: "SERVER", Message: "gzip init failed: " + err.Error(), Exit: 5}
		}
		return &multiCloser{
			Reader:  gr,
			closers: []io.Closer{gr, resp.Body},
		}, nil
	}

	// Plain body — wrap buf (which still contains the peeked bytes) with the body closer.
	return &multiCloser{
		Reader:  buf,
		closers: []io.Closer{resp.Body},
	}, nil
}
