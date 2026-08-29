package endpoints_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bulislaw/suuntool/internal/api"
	"github.com/bulislaw/suuntool/internal/api/endpoints"
)

func TestWhoami_DecodesAskoEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/user", r.URL.Path)
		assert.Equal(t, "SK", r.Header.Get("STTAuthorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"error":null,"payload":{"username":"alice","userKey":"k1"}}`))
	}))
	defer srv.Close()

	client := api.NewClient(srv.URL+"/v1/", "SK", 0)
	user, err := endpoints.Whoami(context.Background(), client)
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "alice", user.Username)
	assert.Equal(t, "k1", user.UserKey)
}
