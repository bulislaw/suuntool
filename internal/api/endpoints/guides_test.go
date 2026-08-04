package endpoints_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tajchert/suuntool/internal/api"
	"github.com/tajchert/suuntool/internal/api/endpoints"
)

func TestListGuides_DecodesAskoEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/suuntoplus/guides/items", r.URL.Path)
		assert.Equal(t, "SK", r.Header.Get("STTAuthorization"))
		// No pagination params — the endpoint accepts none.
		assert.Empty(t, r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"error":null,"payload":[
			{"id":"g1","name":"Easy 40","owner":"alice","fileModificationTime":1700000000000,"pinned":false},
			{"id":"g2","name":"Intervals","owner":"alice","fileModificationTime":1700000100000,"pinned":true,"localDate":"2026-08-03"}
		]}`))
	}))
	defer srv.Close()

	client := api.NewClient(srv.URL+"/v1/", "SK", 0)
	list, err := endpoints.ListGuides(context.Background(), client)
	require.NoError(t, err)
	require.NotNil(t, list)
	require.Len(t, list.Items, 2)
	assert.Equal(t, "g1", list.Items[0].ID)
	assert.Equal(t, "Easy 40", list.Items[0].Name)
	assert.False(t, list.Items[0].Pinned)
	assert.Equal(t, "g2", list.Items[1].ID)
	assert.True(t, list.Items[1].Pinned)
	assert.Equal(t, "2026-08-03", list.Items[1].LocalDate)
}

func TestListGuides_SurfacesAskoError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"error":{"code":500,"description":"boom"},"payload":null}`))
	}))
	defer srv.Close()

	client := api.NewClient(srv.URL+"/v1/", "SK", 0)
	_, err := endpoints.ListGuides(context.Background(), client)
	require.Error(t, err)
	var apiErr *api.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "SERVER", apiErr.Code)
	assert.Equal(t, 5, apiErr.Exit)
}

func TestGuideList_Table(t *testing.T) {
	list := endpoints.GuideList{Items: []endpoints.RemoteGuideInfo{
		{ID: "g1", Name: "Easy 40", FileModificationTime: 1700000000000, Pinned: false},
		{ID: "g2", Name: "Intervals", FileModificationTime: 1700000100000, Pinned: true, LocalDate: "2026-08-03"},
	}}
	headers, rows := list.Table()
	assert.Equal(t, []string{"Modified", "Name", "Pinned", "LocalDate", "ID"}, headers)
	require.Len(t, rows, 2)
	assert.Equal(t, "Easy 40", rows[0][1])
	assert.Equal(t, "g1", rows[0][4])
	assert.Equal(t, "true", rows[1][2])
	assert.Equal(t, "2026-08-03", rows[1][3])
}

func TestDownloadGuide_StreamsRawZipBytes(t *testing.T) {
	zipBytes := []byte("PK\x03\x04fake-zip-content-not-gzip")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/suuntoplus/guides/files/g1", r.URL.Path)
		w.Header().Set("Content-Type", "application/zip")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(zipBytes)
	}))
	defer srv.Close()

	client := api.NewClient(srv.URL+"/v1/", "SK", 0)
	rc, err := endpoints.DownloadGuide(context.Background(), client, "g1")
	require.NoError(t, err)
	defer rc.Close()

	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, zipBytes, got)
}

func TestDownloadGuide_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := api.NewClient(srv.URL+"/v1/", "SK", 0)
	_, err := endpoints.DownloadGuide(context.Background(), client, "missing")
	require.Error(t, err)
	var apiErr *api.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "NOT_FOUND", apiErr.Code)
	assert.Equal(t, 6, apiErr.Exit)
}

func TestCreateGuide_SendsRawZipBodyWithHeaders(t *testing.T) {
	zipBytes := []byte("PK\x03\x04fake-zip-content")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/suuntoplus/guides/files", r.URL.Path)
		assert.Equal(t, "application/zip", r.Header.Get("Content-Type"))
		assert.Equal(t, "5c2fa984-4425-4e72-8f7c-deeaa454b9c6", r.Header.Get("Client-Id"))
		assert.Equal(t, "SK", r.Header.Get("STTAuthorization"))
		// No x-totp on guide writes, unlike comments/reactions/edits.
		assert.Empty(t, r.Header.Get("x-totp"))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.Equal(t, zipBytes, body, "body must be sent raw, byte-for-byte")

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"error":null,"payload":{"id":"g1","name":"Easy 40","owner":"alice","fileModificationTime":1700000000000,"pinned":false}}`))
	}))
	defer srv.Close()

	client := api.NewClient(srv.URL+"/v1/", "SK", 0)
	g, err := endpoints.CreateGuide(context.Background(), client, bytes.NewReader(zipBytes))
	require.NoError(t, err)
	require.NotNil(t, g)
	assert.Equal(t, "g1", g.ID)
}

func TestCreateGuide_DuplicateExternalIdIsExitFive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":{"description":"Conflict"},"payload":null}`))
	}))
	defer srv.Close()

	client := api.NewClient(srv.URL+"/v1/", "SK", 0)
	_, err := endpoints.CreateGuide(context.Background(), client, bytes.NewReader([]byte("PK")))
	require.Error(t, err)
	var apiErr *api.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "SERVER", apiErr.Code)
	assert.Equal(t, 5, apiErr.Exit)
	assert.Contains(t, apiErr.Message, "Conflict")
}

func TestUpdateGuide_PutsToGuideIDPath(t *testing.T) {
	zipBytes := []byte("PK\x03\x04updated-content")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/v1/suuntoplus/guides/files/g1", r.URL.Path)
		assert.Equal(t, "application/zip", r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.Equal(t, zipBytes, body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"error":null,"payload":{"id":"g1","name":"Easy 40 v2","owner":"alice","fileModificationTime":1700000200000,"pinned":false}}`))
	}))
	defer srv.Close()

	client := api.NewClient(srv.URL+"/v1/", "SK", 0)
	g, err := endpoints.UpdateGuide(context.Background(), client, "g1", bytes.NewReader(zipBytes))
	require.NoError(t, err)
	assert.Equal(t, "Easy 40 v2", g.Name)
	assert.Equal(t, int64(1700000200000), g.FileModificationTime)
}

func TestDeleteGuide_SendsDeleteToGuideIDPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/v1/suuntoplus/guides/files/g1", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := api.NewClient(srv.URL+"/v1/", "SK", 0)
	err := endpoints.DeleteGuide(context.Background(), client, "g1")
	require.NoError(t, err)
}

func TestDeleteGuide_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := api.NewClient(srv.URL+"/v1/", "SK", 0)
	err := endpoints.DeleteGuide(context.Background(), client, "missing")
	require.Error(t, err)
	var apiErr *api.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "NOT_FOUND", apiErr.Code)
}

func TestGuideList_Pretty_IncludesCount(t *testing.T) {
	list := endpoints.GuideList{Items: []endpoints.RemoteGuideInfo{
		{ID: "g1", Name: "Easy 40", FileModificationTime: 1700000000000},
	}}
	pretty := list.Pretty()
	assert.Contains(t, pretty, "Easy 40")
	assert.Contains(t, pretty, "1 guide")
}
