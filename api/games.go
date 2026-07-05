package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/josephsae/colombia-ecosystems-engine/domain"
	"github.com/josephsae/colombia-ecosystems-engine/engine"
	"github.com/josephsae/colombia-ecosystems-engine/repository"
)

func (s *Server) createGame(w http.ResponseWriter, r *http.Request) {
	var request CreateGameRequest
	if !s.decodeJSON(w, r, &request) {
		return
	}
	claims := claimsFrom(r.Context())
	count, err := s.repository.CountGames(r.Context(), claims.Subject)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "No fue posible consultar las partidas.", nil)
		return
	}
	if count >= s.config.MaxGames {
		s.writeError(w, r, http.StatusConflict, "GAME_LIMIT_REACHED", "Se alcanzo el limite de partidas.", map[string]any{"limit": s.config.MaxGames})
		return
	}
	state, err := engine.NewGame(s.catalog, domain.NewGameOptions{Seed: request.Seed})
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "No fue posible crear el estado inicial.", nil)
		return
	}
	game, err := s.repository.CreateGame(r.Context(), repository.StoredGame{
		ID: newID("game"), OwnerID: claims.Subject, State: state,
	})
	if err != nil {
		s.writeRepositoryError(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusCreated, gameResponse(game, s.catalog, nil))
}

func (s *Server) listGames(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			s.writeError(w, r, http.StatusBadRequest, "INVALID_QUERY", "limit debe estar entre 1 y 100.", nil)
			return
		}
		limit = parsed
	}
	cursor := r.URL.Query().Get("cursor")
	if cursor != "" && !gameIDPattern.MatchString(cursor) {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_QUERY", "cursor no es valido.", nil)
		return
	}
	page, err := s.repository.ListGames(r.Context(), claimsFrom(r.Context()).Subject, limit, cursor)
	if err != nil {
		s.writeRepositoryError(w, r, err)
		return
	}
	response := ListGamesResponse{NextCursor: page.NextCursor, Games: make([]GameSummary, 0, len(page.Games))}
	for _, game := range page.Games {
		response.Games = append(response.Games, GameSummary{
			ID: game.ID, Version: game.Version, Round: game.State.Round, Phase: game.State.Phase,
			Victory: game.State.Victory, Defeat: game.State.Defeat, CreatedAt: game.CreatedAt, UpdatedAt: game.UpdatedAt,
		})
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) getGame(w http.ResponseWriter, r *http.Request) {
	gameID, ok := s.validGameID(w, r)
	if !ok {
		return
	}
	game, err := s.repository.GetGame(r.Context(), gameID, claimsFrom(r.Context()).Subject)
	if err != nil {
		s.writeRepositoryError(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusOK, gameResponse(game, s.catalog, nil))
}

func (s *Server) applyGameCommand(w http.ResponseWriter, r *http.Request) {
	gameID, ok := s.validGameID(w, r)
	if !ok {
		return
	}
	var request CommandRequest
	if !s.decodeJSON(w, r, &request) {
		return
	}
	if request.ExpectedVersion == 0 || !validCommandRequest(request) {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_COMMAND", "El comando no es valido.", nil)
		return
	}
	ownerID := claimsFrom(r.Context()).Subject
	game, err := s.repository.GetGame(r.Context(), gameID, ownerID)
	if err != nil {
		s.writeRepositoryError(w, r, err)
		return
	}
	if game.Version != request.ExpectedVersion {
		s.writeError(w, r, http.StatusConflict, "VERSION_CONFLICT", "La partida fue modificada por otra solicitud.", map[string]any{"currentVersion": game.Version})
		return
	}
	result, err := engine.Apply(game.State, domain.Command{Type: request.Type, CardID: request.CardID, EventID: request.EventID}, s.catalog)
	if err != nil {
		s.writeEngineError(w, r, err)
		return
	}
	updated, err := s.repository.UpdateGame(r.Context(), gameID, ownerID, request.ExpectedVersion, result.State)
	if err != nil {
		s.writeRepositoryError(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusOK, gameResponse(updated, s.catalog, result.Events))
}

func (s *Server) deleteGame(w http.ResponseWriter, r *http.Request) {
	gameID, ok := s.validGameID(w, r)
	if !ok {
		return
	}
	if err := s.repository.DeleteGame(r.Context(), gameID, claimsFrom(r.Context()).Subject); err != nil {
		s.writeRepositoryError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validCommandRequest(request CommandRequest) bool {
	switch request.Type {
	case domain.PlayCard, domain.DiscardCard:
		return request.CardID != "" && request.EventID == ""
	case domain.ResolveEvent:
		return request.EventID != "" && request.CardID == ""
	case domain.EndTurn:
		return request.CardID == "" && request.EventID == ""
	default:
		return false
	}
}

func (s *Server) validGameID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("gameId")
	if !gameIDPattern.MatchString(id) {
		s.writeError(w, r, http.StatusNotFound, "GAME_NOT_FOUND", "La partida no existe.", nil)
		return "", false
	}
	return id, true
}

func (s *Server) writeEngineError(w http.ResponseWriter, r *http.Request, err error) {
	code := engine.ErrorCode(err)
	status := http.StatusConflict
	if errors.Is(err, engine.ErrInvalidCommand) {
		status = http.StatusBadRequest
	}
	s.writeError(w, r, status, code, err.Error(), nil)
}

func (s *Server) writeRepositoryError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		s.writeError(w, r, http.StatusNotFound, "GAME_NOT_FOUND", "La partida no existe.", nil)
	case errors.Is(err, repository.ErrForbidden):
		s.writeError(w, r, http.StatusForbidden, "FORBIDDEN", "No tienes acceso a esta partida.", nil)
	case errors.Is(err, repository.ErrVersionConflict):
		s.writeError(w, r, http.StatusConflict, "VERSION_CONFLICT", "La partida fue modificada por otra solicitud.", nil)
	default:
		s.writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Ocurrio un error de almacenamiento.", nil)
	}
}
