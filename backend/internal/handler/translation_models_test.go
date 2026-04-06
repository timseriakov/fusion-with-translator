package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/0x2E/fusion/internal/config"
	"github.com/0x2E/fusion/internal/model"
	"github.com/0x2E/fusion/internal/store"
)

type translationModelsEnvelope struct {
	Data translationModelsPayload `json:"data"`
}

type translationModelsPayload struct {
	Models []model.OpenAIModel `json:"models"`
}

type stubTranslationModelsFetcher struct {
	models       []model.OpenAIModel
	err          error
	calls        int
	lastAPIKey   string
	lastCtxIsNil bool
}

func (s *stubTranslationModelsFetcher) Fetch(ctx context.Context, apiKey string) ([]model.OpenAIModel, error) {
	s.calls++
	s.lastAPIKey = apiKey
	s.lastCtxIsNil = ctx == nil
	if s.err != nil {
		return nil, s.err
	}
	return s.models, nil
}

func decodeTranslationModelsResponse(t *testing.T, body []byte) translationModelsEnvelope {
	t.Helper()

	var response translationModelsEnvelope
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return response
}

func TestGetTranslationModels(t *testing.T) {
	t.Run("returns empty list when no api key is configured", func(t *testing.T) {
		h, _ := newTranslationSettingsTestHandler(t, &config.Config{})
		fetcher := &stubTranslationModelsFetcher{}
		h.translationModelsFetcher = fetcher

		w := performRequest(h.SetupRouter(), http.MethodGet, "/api/translation/models", nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		response := decodeTranslationModelsResponse(t, w.Body.Bytes())
		if len(response.Data.Models) != 0 {
			t.Fatalf("expected empty models list, got %d items", len(response.Data.Models))
		}
		if fetcher.calls != 0 {
			t.Fatalf("expected fetcher not to be called without api key, got %d calls", fetcher.calls)
		}
	})

	t.Run("returns empty list when upstream is unavailable", func(t *testing.T) {
		h, st := newTranslationSettingsTestHandler(t, &config.Config{})
		dbKey := "sk-db-secret"
		if _, err := st.UpdateTranslationSettings(store.UpdateTranslationSettingsParams{
			OpenAIAPIKey: &dbKey,
		}); err != nil {
			t.Fatalf("seed translation settings: %v", err)
		}

		fetcher := &stubTranslationModelsFetcher{err: errTranslationModelsUnavailable}
		h.translationModelsFetcher = fetcher

		w := performRequest(h.SetupRouter(), http.MethodGet, "/api/translation/models", nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		response := decodeTranslationModelsResponse(t, w.Body.Bytes())
		if len(response.Data.Models) != 0 {
			t.Fatalf("expected empty models list on upstream failure, got %d items", len(response.Data.Models))
		}
		if fetcher.calls != 1 {
			t.Fatalf("expected fetcher to be called once, got %d calls", fetcher.calls)
		}
		if fetcher.lastAPIKey != dbKey {
			t.Fatalf("expected db api key to be used, got %q", fetcher.lastAPIKey)
		}
		if fetcher.lastCtxIsNil {
			t.Fatal("expected request context to be passed to fetcher")
		}
		if strings.Contains(w.Body.String(), dbKey) {
			t.Fatal("response body leaked db api key")
		}
	})

	t.Run("maps upstream models and uses resolved env key precedence", func(t *testing.T) {
		h, st := newTranslationSettingsTestHandler(t, &config.Config{OpenAIAPIKey: "sk-env-secret"})
		dbKey := "sk-db-secret"
		if _, err := st.UpdateTranslationSettings(store.UpdateTranslationSettingsParams{
			OpenAIAPIKey: &dbKey,
		}); err != nil {
			t.Fatalf("seed translation settings: %v", err)
		}

		fetcher := &stubTranslationModelsFetcher{models: []model.OpenAIModel{
			{ID: "gpt-4.1-mini", Created: 1710000000, Object: "model", OwnedBy: "openai"},
			{ID: "gpt-4o-mini", Created: 1720000000, Object: "model", OwnedBy: "openai"},
		}}
		h.translationModelsFetcher = fetcher

		w := performRequest(h.SetupRouter(), http.MethodGet, "/api/translation/models", nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		response := decodeTranslationModelsResponse(t, w.Body.Bytes())
		if len(response.Data.Models) != 2 {
			t.Fatalf("expected 2 models, got %d", len(response.Data.Models))
		}
		if response.Data.Models[0].ID != "gpt-4.1-mini" {
			t.Fatalf("expected first model id gpt-4.1-mini, got %q", response.Data.Models[0].ID)
		}
		if response.Data.Models[1].OwnedBy != "openai" {
			t.Fatalf("expected second model owned_by openai, got %q", response.Data.Models[1].OwnedBy)
		}
		if fetcher.calls != 1 {
			t.Fatalf("expected fetcher to be called once, got %d calls", fetcher.calls)
		}
		if fetcher.lastAPIKey != "sk-env-secret" {
			t.Fatalf("expected env api key to take precedence, got %q", fetcher.lastAPIKey)
		}
		if strings.Contains(w.Body.String(), "sk-env-secret") || strings.Contains(w.Body.String(), dbKey) {
			t.Fatal("response body leaked an api key")
		}
	})

	t.Run("treats unexpected fetch errors as internal failures", func(t *testing.T) {
		h, st := newTranslationSettingsTestHandler(t, &config.Config{})
		dbKey := "sk-db-secret"
		if _, err := st.UpdateTranslationSettings(store.UpdateTranslationSettingsParams{
			OpenAIAPIKey: &dbKey,
		}); err != nil {
			t.Fatalf("seed translation settings: %v", err)
		}

		h.translationModelsFetcher = &stubTranslationModelsFetcher{err: errors.New("boom")}

		w := performRequest(h.SetupRouter(), http.MethodGet, "/api/translation/models", nil, nil)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "internal server error") {
			t.Fatalf("expected generic internal error body, got %s", w.Body.String())
		}
		if strings.Contains(w.Body.String(), dbKey) {
			t.Fatal("response body leaked db api key")
		}
	})
}
