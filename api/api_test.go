package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/josephsae/colombia-ecosystems-engine/auth"
	"github.com/josephsae/colombia-ecosystems-engine/content"
	"github.com/josephsae/colombia-ecosystems-engine/repository"
)

const testOrigin = "http://localhost:5173"

type apiFixture struct {
	handler http.Handler
}

func newAPIFixture(t *testing.T) apiFixture {
	t.Helper()
	catalog, err := content.LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	repo, err := repository.NewJSONRepository(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	manager, err := auth.NewManager(repo, auth.Config{
		Secret: []byte("0123456789abcdef0123456789abcdef"), Issuer: "test-api", Audience: "test-web",
		AccessTTL: 15 * time.Minute, RefreshTTL: 30 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(repo, manager, catalog, Config{
		AllowedOrigins: []string{testOrigin}, RefreshTTL: 30 * 24 * time.Hour, MaxGames: 20,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, OpenAPISpec())
	if err != nil {
		t.Fatal(err)
	}
	return apiFixture{handler: server.Handler()}
}

func (f apiFixture) guest(t *testing.T) (SessionResponse, *http.Cookie) {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/guest", nil)
	request.Header.Set("Origin", testOrigin)
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create guest status=%d body=%s", response.Code, response.Body.String())
	}
	var session SessionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != refreshCookieName || !cookies[0].HttpOnly {
		t.Fatalf("unexpected refresh cookie: %+v", cookies)
	}
	return session, cookies[0]
}

func (f apiFixture) request(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(data)
	}
	request := httptest.NewRequest(method, path, reader)
	request.Header.Set("Origin", testOrigin)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, request)
	return response
}

func TestHTTPGameFlowOwnershipAndVersioning(t *testing.T) {
	fixture := newAPIFixture(t)
	owner, _ := fixture.guest(t)
	createdResponse := fixture.request(t, http.MethodPost, "/api/v1/games", owner.AccessToken, CreateGameRequest{Seed: 42, Difficulty: "easy"})
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("create game status=%d body=%s", createdResponse.Code, createdResponse.Body.String())
	}
	body := createdResponse.Body.String()
	for _, hidden := range []string{"rngState", `\"deck\":`, `\"queued\":`, "cooldowns"} {
		if strings.Contains(body, hidden) {
			t.Fatalf("public response leaked %q: %s", hidden, body)
		}
	}
	var created GameResponse
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatal(err)
	}
	if created.Version != 1 || created.State.DifficultyID != "easy" || created.State.Resources.Money != 3 || created.State.Cards.DeckCount != 24 || len(created.State.Cards.Hand) != 5 {
		t.Fatalf("unexpected created game: %+v", created)
	}

	command := CommandRequest{Type: "end_turn", ExpectedVersion: 1}
	advancedResponse := fixture.request(t, http.MethodPost, "/api/v1/games/"+created.ID+"/commands", owner.AccessToken, command)
	if advancedResponse.Code != http.StatusOK {
		t.Fatalf("advance status=%d body=%s", advancedResponse.Code, advancedResponse.Body.String())
	}
	var advanced GameResponse
	if err := json.Unmarshal(advancedResponse.Body.Bytes(), &advanced); err != nil {
		t.Fatal(err)
	}
	if advanced.Version != 2 || advanced.State.Round != 1 {
		t.Fatalf("unexpected advanced game: version=%d round=%d", advanced.Version, advanced.State.Round)
	}
	stale := fixture.request(t, http.MethodPost, "/api/v1/games/"+created.ID+"/commands", owner.AccessToken, command)
	if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), "VERSION_CONFLICT") {
		t.Fatalf("stale status=%d body=%s", stale.Code, stale.Body.String())
	}

	other, _ := fixture.guest(t)
	forbidden := fixture.request(t, http.MethodGet, "/api/v1/games/"+created.ID, other.AccessToken, nil)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("cross-owner status=%d body=%s", forbidden.Code, forbidden.Body.String())
	}
	listed := fixture.request(t, http.MethodGet, "/api/v1/games", owner.AccessToken, nil)
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), created.ID) {
		t.Fatalf("list status=%d body=%s", listed.Code, listed.Body.String())
	}
	deleted := fixture.request(t, http.MethodDelete, "/api/v1/games/"+created.ID, owner.AccessToken, nil)
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", deleted.Code, deleted.Body.String())
	}
}

func TestHTTPDifficultyDefaultsValidatesAndAppearsInCatalog(t *testing.T) {
	fixture := newAPIFixture(t)
	owner, _ := fixture.guest(t)
	created := fixture.request(t, http.MethodPost, "/api/v1/games", owner.AccessToken, CreateGameRequest{Seed: 42})
	if created.Code != http.StatusCreated || !strings.Contains(created.Body.String(), `"difficultyId":"easy"`) || !strings.Contains(created.Body.String(), `"money":3`) {
		t.Fatalf("default difficulty status=%d body=%s", created.Code, created.Body.String())
	}
	invalid := fixture.request(t, http.MethodPost, "/api/v1/games", owner.AccessToken, CreateGameRequest{Difficulty: "extreme"})
	if invalid.Code != http.StatusBadRequest || !strings.Contains(invalid.Body.String(), "INVALID_DIFFICULTY") || !strings.Contains(invalid.Body.String(), "easy") {
		t.Fatalf("invalid difficulty status=%d body=%s", invalid.Code, invalid.Body.String())
	}
	catalog := fixture.request(t, http.MethodGet, "/api/v1/catalog", "", nil)
	if catalog.Code != http.StatusOK || !strings.Contains(catalog.Body.String(), `"defaultDifficulty":"easy"`) || !strings.Contains(catalog.Body.String(), `"id":"hard"`) {
		t.Fatalf("catalog status=%d body=%s", catalog.Code, catalog.Body.String())
	}
}

func TestHTTPSessionRefreshCORSAndContentType(t *testing.T) {
	fixture := newAPIFixture(t)
	_, cookie := fixture.guest(t)
	refreshRequest := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/refresh", nil)
	refreshRequest.Header.Set("Origin", testOrigin)
	refreshRequest.AddCookie(cookie)
	refreshResponse := httptest.NewRecorder()
	fixture.handler.ServeHTTP(refreshResponse, refreshRequest)
	if refreshResponse.Code != http.StatusOK {
		t.Fatalf("refresh status=%d body=%s", refreshResponse.Code, refreshResponse.Body.String())
	}
	if refreshResponse.Result().Cookies()[0].Value == cookie.Value {
		t.Fatal("refresh cookie was not rotated")
	}

	replayRequest := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/refresh", nil)
	replayRequest.Header.Set("Origin", testOrigin)
	replayRequest.AddCookie(cookie)
	replayResponse := httptest.NewRecorder()
	fixture.handler.ServeHTTP(replayResponse, replayRequest)
	if replayResponse.Code != http.StatusUnauthorized || !strings.Contains(replayResponse.Body.String(), "REFRESH_REUSED") {
		t.Fatalf("replay status=%d body=%s", replayResponse.Code, replayResponse.Body.String())
	}

	badOrigin := httptest.NewRequest(http.MethodGet, "/api/v1/catalog", nil)
	badOrigin.Header.Set("Origin", "https://malicious.example")
	badOriginResponse := httptest.NewRecorder()
	fixture.handler.ServeHTTP(badOriginResponse, badOrigin)
	if badOriginResponse.Code != http.StatusForbidden {
		t.Fatalf("bad origin status=%d", badOriginResponse.Code)
	}

	freshSession, _ := fixture.guest(t)
	unsupported := httptest.NewRequest(http.MethodPost, "/api/v1/games", strings.NewReader(`{}`))
	unsupported.Header.Set("Origin", testOrigin)
	unsupported.Header.Set("Authorization", "Bearer "+freshSession.AccessToken)
	unsupportedResponse := httptest.NewRecorder()
	fixture.handler.ServeHTTP(unsupportedResponse, unsupported)
	if unsupportedResponse.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("unsupported media status=%d body=%s", unsupportedResponse.Code, unsupportedResponse.Body.String())
	}
}

func TestPublicCatalogHealthAndDocumentation(t *testing.T) {
	fixture := newAPIFixture(t)
	for path, expected := range map[string]int{
		"/api/v1/catalog": http.StatusOK, "/health/live": http.StatusOK, "/health/ready": http.StatusOK,
		"/openapi.yaml": http.StatusOK, "/docs/": http.StatusOK,
	} {
		response := fixture.request(t, http.MethodGet, path, "", nil)
		if response.Code != expected {
			t.Errorf("GET %s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}

func TestHTTPConcurrentCommandsOnlyApplyOnce(t *testing.T) {
	fixture := newAPIFixture(t)
	owner, _ := fixture.guest(t)
	createdResponse := fixture.request(t, http.MethodPost, "/api/v1/games", owner.AccessToken, CreateGameRequest{Seed: 42})
	var created GameResponse
	if err := json.Unmarshal(createdResponse.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	statuses := make(chan int, 2)
	var wait sync.WaitGroup
	for i := 0; i < 2; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			response := fixture.request(t, http.MethodPost, "/api/v1/games/"+created.ID+"/commands", owner.AccessToken,
				CommandRequest{Type: "end_turn", ExpectedVersion: 1})
			statuses <- response.Code
		}()
	}
	wait.Wait()
	close(statuses)
	counts := map[int]int{}
	for status := range statuses {
		counts[status]++
	}
	if counts[http.StatusOK] != 1 || counts[http.StatusConflict] != 1 {
		t.Fatalf("concurrent statuses = %+v", counts)
	}
}

func TestHTTPSessionCurrentAndLogout(t *testing.T) {
	fixture := newAPIFixture(t)
	session, _ := fixture.guest(t)
	current := fixture.request(t, http.MethodGet, "/api/v1/sessions/current", session.AccessToken, nil)
	if current.Code != http.StatusOK || !strings.Contains(current.Body.String(), session.GuestID) {
		t.Fatalf("current status=%d body=%s", current.Code, current.Body.String())
	}
	logout := fixture.request(t, http.MethodDelete, "/api/v1/sessions/current", session.AccessToken, nil)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout status=%d body=%s", logout.Code, logout.Body.String())
	}
	after := fixture.request(t, http.MethodGet, "/api/v1/sessions/current", session.AccessToken, nil)
	if after.Code != http.StatusUnauthorized {
		t.Fatalf("revoked access status=%d body=%s", after.Code, after.Body.String())
	}
}

func TestHTTPCardLookupAndNotFound(t *testing.T) {
	fixture := newAPIFixture(t)
	found := fixture.request(t, http.MethodGet, "/api/v1/catalog/cards/livestock", "", nil)
	if found.Code != http.StatusOK || !strings.Contains(found.Body.String(), "Ganaderia") {
		t.Fatalf("card status=%d body=%s", found.Code, found.Body.String())
	}
	missing := fixture.request(t, http.MethodGet, "/api/v1/catalog/cards/missing", "", nil)
	if missing.Code != http.StatusNotFound || !strings.Contains(missing.Body.String(), "CARD_NOT_FOUND") {
		t.Fatalf("missing card status=%d body=%s", missing.Code, missing.Body.String())
	}
}
