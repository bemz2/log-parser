package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	transport "log-parser/internal/delivery/http"
	"log-parser/internal/domain"
	"log-parser/internal/service"
)

type TopologyService interface {
	Parse(ctx context.Context, requestedPath string) (service.ParseResult, error)
	GetTopology(ctx context.Context, logID int64) (domain.Topology, error)
	GetNode(ctx context.Context, nodeID int64) (domain.Node, error)
	GetPortsByNode(ctx context.Context, nodeID int64) ([]domain.Port, error)
	GetLog(ctx context.Context, logID int64) (domain.Log, error)
}

type TopologyHandler struct {
	service TopologyService
}

func NewTopologyHandler(service TopologyService) *TopologyHandler {
	return &TopologyHandler{service: service}
}

func (h *TopologyHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/parse", h.Parse)
	mux.HandleFunc("GET /api/v1/topology/{log_id}", h.GetTopology)
	mux.HandleFunc("GET /api/v1/node/{node_id}", h.GetNode)
	mux.HandleFunc("GET /api/v1/port/{node_id}", h.GetPortsByNode)
	mux.HandleFunc("GET /api/v1/log/{log_id}", h.GetLog)
}

type parseRequest struct {
	Path string `json:"path"`
}

func (h *TopologyHandler) Parse(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()

	var req parseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		transport.RespondError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	result, err := h.service.Parse(r.Context(), req.Path)
	if err != nil {
		if errors.Is(err, service.ErrInvalidPath) {
			transport.RespondError(w, http.StatusBadRequest, "invalid file path")
			return
		}
		if result.LogID > 0 {
			transport.RespondJSON(w, http.StatusUnprocessableEntity, result)
			return
		}
		transport.RespondError(w, http.StatusInternalServerError, "failed to parse log")
		return
	}

	transport.RespondJSON(w, http.StatusCreated, result)
}

func (h *TopologyHandler) GetTopology(w http.ResponseWriter, r *http.Request) {
	logID, ok := pathID(w, r, "log_id")
	if !ok {
		return
	}

	topology, err := h.service.GetTopology(r.Context(), logID)
	respondResult(w, topology, err)
}

func (h *TopologyHandler) GetNode(w http.ResponseWriter, r *http.Request) {
	nodeID, ok := pathID(w, r, "node_id")
	if !ok {
		return
	}

	node, err := h.service.GetNode(r.Context(), nodeID)
	respondResult(w, node, err)
}

func (h *TopologyHandler) GetPortsByNode(w http.ResponseWriter, r *http.Request) {
	nodeID, ok := pathID(w, r, "node_id")
	if !ok {
		return
	}

	ports, err := h.service.GetPortsByNode(r.Context(), nodeID)
	respondResult(w, map[string]any{"node_id": nodeID, "ports": ports}, err)
}

func (h *TopologyHandler) GetLog(w http.ResponseWriter, r *http.Request) {
	logID, ok := pathID(w, r, "log_id")
	if !ok {
		return
	}

	log, err := h.service.GetLog(r.Context(), logID)
	respondResult(w, log, err)
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	raw := strings.TrimSpace(r.PathValue(name))
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		transport.RespondError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

func respondResult(w http.ResponseWriter, payload any, err error) {
	if errors.Is(err, service.ErrNotFound) {
		transport.RespondError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		transport.RespondError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	transport.RespondJSON(w, http.StatusOK, payload)
}
