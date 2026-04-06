package handler

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0x2E/fusion/internal/config"
	"github.com/0x2E/fusion/internal/store"
)

type translationSettingsEnvelope struct {
	Data translationSettingsPayload `json:"data"`
}

type translationSettingsPayload struct {
	HasAPIKey                 bool   `json:"has_api_key"`
	MaskedAPIKey              string `json:"masked_api_key"`
	APIKeySource              string `json:"api_key_source"`
	TranslationModel          string `json:"translation_model"`
	TranslationTargetLanguage string `json:"translation_target_language"`
}

func newTranslationSettingsTestHandler(t *testing.T, cfg *config.Config) (*Handler, *store.Store) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	if cfg == nil {
		cfg = &config.Config{}
	}

	h := &Handler{
		store:        st,
		config:       cfg,
		allowAnonAPI: true,
		sessions:     make(map[string]int64),
		limiter:      newLoginLimiter(10, 60, 300),
	}

	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})

	return h, st
}

func decodeTranslationSettingsResponse(t *testing.T, body []byte) translationSettingsEnvelope {
	t.Helper()

	var response translationSettingsEnvelope
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return response
}

func TestGetTranslationSettings(t *testing.T) {
	t.Run("returns env-backed key metadata without leaking the secret", func(t *testing.T) {
		h, _ := newTranslationSettingsTestHandler(t, &config.Config{OpenAIAPIKey: "sk-env-secret"})

		w := performRequest(h.SetupRouter(), http.MethodGet, "/api/translation/settings", nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		response := decodeTranslationSettingsResponse(t, w.Body.Bytes())
		if !response.Data.HasAPIKey {
			t.Fatal("expected has_api_key to be true")
		}
		if response.Data.APIKeySource != "env" {
			t.Fatalf("expected api_key_source env, got %q", response.Data.APIKeySource)
		}
		if response.Data.MaskedAPIKey == "" {
			t.Fatal("expected masked_api_key to be populated")
		}
		if response.Data.MaskedAPIKey == "sk-env-secret" {
			t.Fatal("masked_api_key leaked env secret")
		}
		if strings.Contains(w.Body.String(), "sk-env-secret") {
			t.Fatal("response body leaked env secret")
		}
	})

	t.Run("returns db-backed key metadata without leaking the secret", func(t *testing.T) {
		h, st := newTranslationSettingsTestHandler(t, &config.Config{})
		dbKey := "sk-db-secret"
		model := "gpt-4o-mini"
		language := "ru"
		if _, err := st.UpdateTranslationSettings(store.UpdateTranslationSettingsParams{
			OpenAIAPIKey:              &dbKey,
			TranslationModel:          &model,
			TranslationTargetLanguage: &language,
		}); err != nil {
			t.Fatalf("seed translation settings: %v", err)
		}

		w := performRequest(h.SetupRouter(), http.MethodGet, "/api/translation/settings", nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		response := decodeTranslationSettingsResponse(t, w.Body.Bytes())
		if !response.Data.HasAPIKey {
			t.Fatal("expected has_api_key to be true")
		}
		if response.Data.APIKeySource != "db" {
			t.Fatalf("expected api_key_source db, got %q", response.Data.APIKeySource)
		}
		if response.Data.MaskedAPIKey == "" {
			t.Fatal("expected masked_api_key to be populated")
		}
		if response.Data.MaskedAPIKey == dbKey {
			t.Fatal("masked_api_key leaked db secret")
		}
		if response.Data.TranslationModel != model {
			t.Fatalf("expected translation_model %q, got %q", model, response.Data.TranslationModel)
		}
		if response.Data.TranslationTargetLanguage != language {
			t.Fatalf("expected translation_target_language %q, got %q", language, response.Data.TranslationTargetLanguage)
		}
		if strings.Contains(w.Body.String(), dbKey) {
			t.Fatal("response body leaked db secret")
		}
	})

	t.Run("env key overrides db fallback at runtime", func(t *testing.T) {
		h, st := newTranslationSettingsTestHandler(t, &config.Config{OpenAIAPIKey: "sk-env-secret"})
		dbKey := "sk-db-secret"
		model := "gpt-4.1-mini"
		language := "de"
		if _, err := st.UpdateTranslationSettings(store.UpdateTranslationSettingsParams{
			OpenAIAPIKey:              &dbKey,
			TranslationModel:          &model,
			TranslationTargetLanguage: &language,
		}); err != nil {
			t.Fatalf("seed translation settings: %v", err)
		}

		w := performRequest(h.SetupRouter(), http.MethodGet, "/api/translation/settings", nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		response := decodeTranslationSettingsResponse(t, w.Body.Bytes())
		if !response.Data.HasAPIKey {
			t.Fatal("expected has_api_key to be true")
		}
		if response.Data.APIKeySource != "env" {
			t.Fatalf("expected api_key_source env, got %q", response.Data.APIKeySource)
		}
		if response.Data.MaskedAPIKey == "sk-env-secret" || response.Data.MaskedAPIKey == dbKey {
			t.Fatal("masked_api_key leaked a raw secret")
		}
		if response.Data.TranslationModel != model {
			t.Fatalf("expected translation_model %q, got %q", model, response.Data.TranslationModel)
		}
		if response.Data.TranslationTargetLanguage != language {
			t.Fatalf("expected translation_target_language %q, got %q", language, response.Data.TranslationTargetLanguage)
		}
	})
}

func TestPatchTranslationSettings(t *testing.T) {
	t.Run("supports partial updates without overwriting unspecified fields", func(t *testing.T) {
		h, st := newTranslationSettingsTestHandler(t, &config.Config{})
		dbKey := "sk-db-secret"
		model := "gpt-4o-mini"
		language := "ru"
		if _, err := st.UpdateTranslationSettings(store.UpdateTranslationSettingsParams{
			OpenAIAPIKey:              &dbKey,
			TranslationModel:          &model,
			TranslationTargetLanguage: &language,
		}); err != nil {
			t.Fatalf("seed translation settings: %v", err)
		}

		w := performRequest(
			h.SetupRouter(),
			http.MethodPatch,
			"/api/translation/settings",
			mustJSONBody(t, map[string]string{"translation_target_language": "fr"}),
			map[string]string{"Content-Type": "application/json"},
		)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		response := decodeTranslationSettingsResponse(t, w.Body.Bytes())
		if response.Data.TranslationModel != model {
			t.Fatalf("expected translation_model %q, got %q", model, response.Data.TranslationModel)
		}
		if response.Data.TranslationTargetLanguage != "fr" {
			t.Fatalf("expected translation_target_language fr, got %q", response.Data.TranslationTargetLanguage)
		}
		if response.Data.APIKeySource != "db" {
			t.Fatalf("expected api_key_source db, got %q", response.Data.APIKeySource)
		}

		settings, err := st.GetTranslationSettings()
		if err != nil {
			t.Fatalf("reload translation settings: %v", err)
		}
		if settings.OpenAIAPIKey != dbKey {
			t.Fatalf("expected db api key %q, got %q", dbKey, settings.OpenAIAPIKey)
		}
		if settings.TranslationModel != model {
			t.Fatalf("expected stored translation model %q, got %q", model, settings.TranslationModel)
		}
		if settings.TranslationTargetLanguage != "fr" {
			t.Fatalf("expected stored target language fr, got %q", settings.TranslationTargetLanguage)
		}
	})

	t.Run("empty string clears the db fallback key", func(t *testing.T) {
		h, st := newTranslationSettingsTestHandler(t, &config.Config{})
		dbKey := "sk-db-secret"
		if _, err := st.UpdateTranslationSettings(store.UpdateTranslationSettingsParams{
			OpenAIAPIKey: &dbKey,
		}); err != nil {
			t.Fatalf("seed translation settings: %v", err)
		}

		w := performRequest(
			h.SetupRouter(),
			http.MethodPatch,
			"/api/translation/settings",
			mustJSONBody(t, map[string]string{"openai_api_key": ""}),
			map[string]string{"Content-Type": "application/json"},
		)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		response := decodeTranslationSettingsResponse(t, w.Body.Bytes())
		if response.Data.HasAPIKey {
			t.Fatal("expected has_api_key to be false after clearing the db key")
		}
		if response.Data.MaskedAPIKey != "" {
			t.Fatalf("expected empty masked_api_key after clear, got %q", response.Data.MaskedAPIKey)
		}
		if response.Data.APIKeySource != "" {
			t.Fatalf("expected empty api_key_source after clear, got %q", response.Data.APIKeySource)
		}

		settings, err := st.GetTranslationSettings()
		if err != nil {
			t.Fatalf("reload translation settings: %v", err)
		}
		if settings.OpenAIAPIKey != "" {
			t.Fatalf("expected cleared db api key, got %q", settings.OpenAIAPIKey)
		}
	})

	t.Run("env key remains the runtime source while patch updates the db fallback", func(t *testing.T) {
		h, st := newTranslationSettingsTestHandler(t, &config.Config{OpenAIAPIKey: "sk-env-secret"})
		oldDBKey := "sk-db-old"
		if _, err := st.UpdateTranslationSettings(store.UpdateTranslationSettingsParams{
			OpenAIAPIKey: &oldDBKey,
		}); err != nil {
			t.Fatalf("seed translation settings: %v", err)
		}

		w := performRequest(
			h.SetupRouter(),
			http.MethodPatch,
			"/api/translation/settings",
			mustJSONBody(t, map[string]string{"openai_api_key": "sk-db-new", "translation_model": "gpt-4.1-mini"}),
			map[string]string{"Content-Type": "application/json"},
		)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		response := decodeTranslationSettingsResponse(t, w.Body.Bytes())
		if !response.Data.HasAPIKey {
			t.Fatal("expected has_api_key to stay true with env key configured")
		}
		if response.Data.APIKeySource != "env" {
			t.Fatalf("expected api_key_source env, got %q", response.Data.APIKeySource)
		}
		if response.Data.MaskedAPIKey == "sk-env-secret" || response.Data.MaskedAPIKey == "sk-db-new" {
			t.Fatal("masked_api_key leaked a raw secret")
		}
		if response.Data.TranslationModel != "gpt-4.1-mini" {
			t.Fatalf("expected translation_model gpt-4.1-mini, got %q", response.Data.TranslationModel)
		}

		settings, err := st.GetTranslationSettings()
		if err != nil {
			t.Fatalf("reload translation settings: %v", err)
		}
		if settings.OpenAIAPIKey != "sk-db-new" {
			t.Fatalf("expected db fallback key to update, got %q", settings.OpenAIAPIKey)
		}
		if settings.TranslationModel != "gpt-4.1-mini" {
			t.Fatalf("expected stored translation model gpt-4.1-mini, got %q", settings.TranslationModel)
		}
	})
}
