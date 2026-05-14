package http

import "net/http"

func RegisterHealthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}
