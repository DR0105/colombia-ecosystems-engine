package api

import (
	"net/http"

	apidocs "github.com/josephsae/colombia-ecosystems-engine/docs/api"
)

func (s *Server) getCatalog(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, catalogResponse(s.catalog))
}

func (s *Server) getCard(w http.ResponseWriter, r *http.Request) {
	card, ok := s.catalog.Cards[r.PathValue("cardId")]
	if !ok {
		s.writeError(w, r, http.StatusNotFound, "CARD_NOT_FOUND", "La carta no existe.", nil)
		return
	}
	s.writeJSON(w, http.StatusOK, card)
}

func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if len(s.catalog.Cards) == 0 || s.repository.Ready(r.Context()) != nil {
		s.writeError(w, r, http.StatusServiceUnavailable, "NOT_READY", "El servicio no esta listo.", nil)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) openAPISpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(s.openAPI)
}

func (s *Server) swaggerUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/docs/" {
		http.Redirect(w, r, "/docs/", http.StatusPermanentRedirect)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(apidocs.SwaggerHTML())
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	s.writeError(w, r, http.StatusNotFound, "NOT_FOUND", "El recurso no existe.", nil)
}

func OpenAPISpec() []byte {
	return apidocs.OpenAPI()
}
