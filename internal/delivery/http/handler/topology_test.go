package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	transport "log-parser/internal/delivery/http"
	"log-parser/internal/delivery/http/handler"
	"log-parser/internal/domain"
	"log-parser/internal/service"
	"log-parser/mocks"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestTopologyHandlerParseSuccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := mocks.NewTopologyService(t)
	mux := testMux(svc)

	svc.EXPECT().
		Parse(mock.Anything, "data/example.log").
		Return(service.ParseResult{LogID: 1, Status: domain.LogStatusParsed}, nil).
		Once()

	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/parse", bytes.NewBufferString(`{"path":"data/example.log"}`))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.JSONEq(t, `{"log_id":1,"status":"parsed"}`, rec.Body.String())
}

func TestTopologyHandlerParseInvalidPath(t *testing.T) {
	t.Parallel()

	svc := mocks.NewTopologyService(t)
	mux := testMux(svc)

	svc.EXPECT().
		Parse(mock.Anything, "../secret.log").
		Return(service.ParseResult{}, service.ErrInvalidPath).
		Once()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/parse", bytes.NewBufferString(`{"path":"../secret.log"}`))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"error":"invalid file path"}`, rec.Body.String())
}

func TestTopologyHandlerGetTopology(t *testing.T) {
	t.Parallel()

	svc := mocks.NewTopologyService(t)
	mux := testMux(svc)
	expected := domain.Topology{
		LogID: 1,
		Nodes: []domain.Node{
			{
				ID:   2,
				Name: "switch-1",
				Type: "switch",
				Ports: []domain.Port{
					{ID: 3, Name: "eth0", IP: "10.0.0.1", Status: "active"},
				},
			},
		},
	}

	svc.EXPECT().
		GetTopology(mock.Anything, int64(1)).
		Return(expected, nil).
		Once()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/topology/1", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, mustJSON(t, expected), rec.Body.String())
}

func TestTopologyHandlerGetNodeNotFound(t *testing.T) {
	t.Parallel()

	svc := mocks.NewTopologyService(t)
	mux := testMux(svc)

	svc.EXPECT().
		GetNode(mock.Anything, int64(404)).
		Return(domain.Node{}, service.ErrNotFound).
		Once()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/node/404", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.JSONEq(t, `{"error":"not found"}`, rec.Body.String())
}

func TestTopologyHandlerGetPortsByNode(t *testing.T) {
	t.Parallel()

	svc := mocks.NewTopologyService(t)
	mux := testMux(svc)
	ports := []domain.Port{{ID: 1, NodeID: 2, Name: "eth0", Status: "active"}}

	svc.EXPECT().
		GetPortsByNode(mock.Anything, int64(2)).
		Return(ports, nil).
		Once()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/port/2", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"node_id":2,"ports":[{"id":1,"node_id":2,"name":"eth0","status":"active"}]}`, rec.Body.String())
}

func TestTopologyHandlerGetLog(t *testing.T) {
	t.Parallel()

	svc := mocks.NewTopologyService(t)
	mux := testMux(svc)
	uploadedAt := time.Date(2026, time.May, 15, 10, 0, 0, 0, time.UTC)
	log := domain.Log{ID: 1, FilePath: "data/example.log", Status: domain.LogStatusParsed, NodesCount: 2, PortsCount: 3, UploadedAt: uploadedAt}

	svc.EXPECT().
		GetLog(mock.Anything, int64(1)).
		Return(log, nil).
		Once()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/log/1", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, mustJSON(t, log), rec.Body.String())
}

func TestTopologyHandlerRejectsInvalidID(t *testing.T) {
	t.Parallel()

	svc := mocks.NewTopologyService(t)
	mux := testMux(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/log/abc", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"error":"invalid id"}`, rec.Body.String())
	svc.AssertNotCalled(t, "GetLog", mock.Anything, mock.Anything)
}

func TestTopologyHandlerReturnsInternalError(t *testing.T) {
	t.Parallel()

	svc := mocks.NewTopologyService(t)
	mux := testMux(svc)

	svc.EXPECT().
		GetLog(mock.Anything, int64(1)).
		Return(domain.Log{}, errors.New("db down")).
		Once()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/log/1", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.JSONEq(t, `{"error":"internal server error"}`, rec.Body.String())
}

func testMux(svc *mocks.TopologyService) *http.ServeMux {
	mux := http.NewServeMux()
	transport.RegisterHealthRoutes(mux)
	handler.NewTopologyHandler(svc).Register(mux)
	return mux
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()

	payload, err := json.Marshal(value)
	require.NoError(t, err)
	return string(payload)
}
