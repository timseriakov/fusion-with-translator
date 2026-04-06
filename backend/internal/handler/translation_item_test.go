package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/0x2E/fusion/internal/config"
	"github.com/0x2E/fusion/internal/store"
)

type translateItemEnvelope struct {
	Data translateItemPayload `json:"data"`
}

type translateItemPayload struct {
	ID                        int64   `json:"id"`
	Title                     string  `json:"title"`
	Content                   string  `json:"content"`
	TranslatedTitle           *string `json:"translated_title"`
	TranslatedContent         *string `json:"translated_content"`
	TranslationModel          string  `json:"translation_model"`
	TranslationTargetLanguage string  `json:"translation_target_language"`
	TranslationUpdatedAt      int64   `json:"translation_updated_at"`
}

type stubItemTranslator struct {
	responses []stubTranslationResult
	calls     []stubTranslationCall
}

type stubTranslationResult struct {
	output string
	err    error
}

type stubTranslationCall struct {
	apiKey       string
	model        string
	systemPrompt string
	userPrompt   string
}

func (s *stubItemTranslator) Translate(ctx context.Context, apiKey, model, systemPrompt, userPrompt string) (string, error) {
	_ = ctx
	s.calls = append(s.calls, stubTranslationCall{
		apiKey:       apiKey,
		model:        model,
		systemPrompt: systemPrompt,
		userPrompt:   userPrompt,
	})
	if len(s.responses) == 0 {
		return "", errors.New("unexpected translation call")
	}
	result := s.responses[0]
	s.responses = s.responses[1:]
	return result.output, result.err
}

func newTranslationItemTestHandler(t *testing.T, cfg *config.Config) (*Handler, *store.Store) {
	t.Helper()
	return newTranslationSettingsTestHandler(t, cfg)
}

func decodeTranslateItemResponse(t *testing.T, body []byte) translateItemEnvelope {
	t.Helper()

	var response translateItemEnvelope
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return response
}

func seedTranslationItemFixture(t *testing.T, st *store.Store, title string, content string) int64 {
	t.Helper()

	group, err := st.CreateGroup("Translation Group")
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	feed, err := st.CreateFeed(group.ID, "Translation Feed", "https://example.com/feed.xml", "https://example.com", "")
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}
	item, err := st.CreateItem(feed.ID, fmt.Sprintf("guid-%s", title), title, "https://example.com/item", content, 1700000000)
	if err != nil {
		t.Fatalf("create item: %v", err)
	}
	return item.ID
}

func mustSeedTranslationSettings(t *testing.T, st *store.Store, apiKey, model, language string) {
	t.Helper()
	if _, err := st.UpdateTranslationSettings(store.UpdateTranslationSettingsParams{
		OpenAIAPIKey:              &apiKey,
		TranslationModel:          &model,
		TranslationTargetLanguage: &language,
	}); err != nil {
		t.Fatalf("seed translation settings: %v", err)
	}
}

func TestTranslateItem(t *testing.T) {
	t.Run("returns cached translation for matching model and language", func(t *testing.T) {
		h, st := newTranslationItemTestHandler(t, &config.Config{})
		mustSeedTranslationSettings(t, st, "sk-db-secret", "gpt-4o-mini", "ru")
		itemID := seedTranslationItemFixture(t, st, "Original title", "<p>Hello <strong>world</strong></p>")

		cachedTitle := "Кэшированный заголовок"
		cachedContent := "<p>Привет <strong>мир</strong></p>"
		if err := st.SaveItemTranslation(itemID, store.SaveItemTranslationInput{
			TranslatedTitle:           &cachedTitle,
			TranslatedContent:         &cachedContent,
			TranslationModel:          "gpt-4o-mini",
			TranslationTargetLanguage: "ru",
			TranslationUpdatedAt:      1710000000,
		}); err != nil {
			t.Fatalf("seed cached translation: %v", err)
		}

		translator := &stubItemTranslator{}
		h.itemTranslator = translator

		w := performRequest(h.SetupRouter(), http.MethodPost, fmt.Sprintf("/api/translation/items/%d", itemID), nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d body=%s", w.Code, w.Body.String())
		}
		if len(translator.calls) != 0 {
			t.Fatalf("expected no translator calls on cache hit, got %d", len(translator.calls))
		}

		response := decodeTranslateItemResponse(t, w.Body.Bytes())
		if response.Data.TranslatedTitle == nil || *response.Data.TranslatedTitle != cachedTitle {
			t.Fatalf("expected cached translated title %q, got %#v", cachedTitle, response.Data.TranslatedTitle)
		}
		if response.Data.TranslatedContent == nil || *response.Data.TranslatedContent != cachedContent {
			t.Fatalf("expected cached translated content %q, got %#v", cachedContent, response.Data.TranslatedContent)
		}
		if response.Data.TranslationUpdatedAt != 1710000000 {
			t.Fatalf("expected cached translation_updated_at 1710000000, got %d", response.Data.TranslationUpdatedAt)
		}
	})

	t.Run("force refresh bypasses cache and persists a new translation", func(t *testing.T) {
		h, st := newTranslationItemTestHandler(t, &config.Config{})
		mustSeedTranslationSettings(t, st, "sk-db-secret", "gpt-4o-mini", "ru")
		itemID := seedTranslationItemFixture(t, st, "Original title", "<p>Hello <strong>world</strong></p>")

		cachedTitle := "Старый заголовок"
		cachedContent := "<p>Старый <strong>контент</strong></p>"
		if err := st.SaveItemTranslation(itemID, store.SaveItemTranslationInput{
			TranslatedTitle:           &cachedTitle,
			TranslatedContent:         &cachedContent,
			TranslationModel:          "gpt-4o-mini",
			TranslationTargetLanguage: "ru",
			TranslationUpdatedAt:      1710000000,
		}); err != nil {
			t.Fatalf("seed cached translation: %v", err)
		}

		translator := &stubItemTranslator{responses: []stubTranslationResult{
			{output: "Новый заголовок"},
			{output: "[0] Привет\n[1] мир"},
		}}
		h.itemTranslator = translator

		w := performRequest(h.SetupRouter(), http.MethodPost, fmt.Sprintf("/api/translation/items/%d?force=true", itemID), nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d body=%s", w.Code, w.Body.String())
		}
		if len(translator.calls) != 2 {
			t.Fatalf("expected 2 translator calls on force refresh, got %d", len(translator.calls))
		}

		response := decodeTranslateItemResponse(t, w.Body.Bytes())
		if response.Data.TranslatedTitle == nil || *response.Data.TranslatedTitle != "Новый заголовок" {
			t.Fatalf("expected refreshed translated title, got %#v", response.Data.TranslatedTitle)
		}
		if response.Data.TranslatedContent == nil {
			t.Fatal("expected refreshed translated content to be present")
		}
		if !validateHTMLStructure("<p>Hello <strong>world</strong></p>", *response.Data.TranslatedContent) {
			t.Fatalf("expected translated content to preserve structure, got %q", *response.Data.TranslatedContent)
		}
		if !strings.Contains(*response.Data.TranslatedContent, "Привет") || !strings.Contains(*response.Data.TranslatedContent, "мир") {
			t.Fatalf("expected translated content text to be updated, got %q", *response.Data.TranslatedContent)
		}
		if response.Data.TranslationUpdatedAt <= 1710000000 {
			t.Fatalf("expected translation_updated_at to increase, got %d", response.Data.TranslationUpdatedAt)
		}
	})

	t.Run("empty content results in title-only translation", func(t *testing.T) {
		h, st := newTranslationItemTestHandler(t, &config.Config{})
		mustSeedTranslationSettings(t, st, "sk-db-secret", "gpt-4o-mini", "ru")
		itemID := seedTranslationItemFixture(t, st, "Original title", "")

		translator := &stubItemTranslator{responses: []stubTranslationResult{{output: "Перевод заголовка"}}}
		h.itemTranslator = translator

		w := performRequest(h.SetupRouter(), http.MethodPost, fmt.Sprintf("/api/translation/items/%d", itemID), nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d body=%s", w.Code, w.Body.String())
		}
		if len(translator.calls) != 1 {
			t.Fatalf("expected only title translation call, got %d", len(translator.calls))
		}

		response := decodeTranslateItemResponse(t, w.Body.Bytes())
		if response.Data.TranslatedTitle == nil || *response.Data.TranslatedTitle != "Перевод заголовка" {
			t.Fatalf("expected title-only translation, got %#v", response.Data.TranslatedTitle)
		}
		if response.Data.TranslatedContent != nil {
			t.Fatalf("expected translated content to stay nil, got %#v", response.Data.TranslatedContent)
		}
	})

	t.Run("plain text content is translated directly", func(t *testing.T) {
		h, st := newTranslationItemTestHandler(t, &config.Config{})
		mustSeedTranslationSettings(t, st, "sk-db-secret", "gpt-4o-mini", "ru")
		itemID := seedTranslationItemFixture(t, st, "Original title", "Just plain text, no HTML tags here.")

		translator := &stubItemTranslator{responses: []stubTranslationResult{
			{output: "Перевод заголовка"},
			{output: "Просто обычный текст, без HTML тегов."},
		}}
		h.itemTranslator = translator

		w := performRequest(h.SetupRouter(), http.MethodPost, fmt.Sprintf("/api/translation/items/%d", itemID), nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d body=%s", w.Code, w.Body.String())
		}
		if len(translator.calls) != 2 {
			t.Fatalf("expected 2 translation calls (title + content), got %d", len(translator.calls))
		}

		response := decodeTranslateItemResponse(t, w.Body.Bytes())
		if response.Data.TranslatedTitle == nil || *response.Data.TranslatedTitle != "Перевод заголовка" {
			t.Fatalf("expected translated title, got %#v", response.Data.TranslatedTitle)
		}
		if response.Data.TranslatedContent == nil || *response.Data.TranslatedContent != "Просто обычный текст, без HTML тегов." {
			t.Fatalf("expected translated plain text content, got %#v", response.Data.TranslatedContent)
		}
	})

	t.Run("rejects structurally invalid translated html and does not save", func(t *testing.T) {
		h, st := newTranslationItemTestHandler(t, &config.Config{})
		mustSeedTranslationSettings(t, st, "sk-db-secret", "gpt-4o-mini", "ru")
		itemID := seedTranslationItemFixture(t, st, "Original title", "<p>Hello <strong>world</strong></p>")

		translator := &stubItemTranslator{responses: []stubTranslationResult{
			{output: "Перевод заголовка"},
			{output: "[0] <div>сломано</div>\n[1] мир"},
		}}
		h.itemTranslator = translator

		w := performRequest(h.SetupRouter(), http.MethodPost, fmt.Sprintf("/api/translation/items/%d", itemID), nil, nil)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d body=%s", w.Code, w.Body.String())
		}

		item, err := st.GetItem(itemID)
		if err != nil {
			t.Fatalf("reload item: %v", err)
		}
		if item.TranslatedTitle != nil || item.TranslatedContent != nil || item.TranslationUpdatedAt != 0 {
			t.Fatalf("expected no translation to be saved after invalid html, got %+v", item)
		}
	})

	t.Run("rejects when both translated fields are empty and does not save", func(t *testing.T) {
		h, st := newTranslationItemTestHandler(t, &config.Config{})
		mustSeedTranslationSettings(t, st, "sk-db-secret", "gpt-4o-mini", "ru")
		itemID := seedTranslationItemFixture(t, st, "Original title", "")

		translator := &stubItemTranslator{responses: []stubTranslationResult{{output: "   "}}}
		h.itemTranslator = translator

		w := performRequest(h.SetupRouter(), http.MethodPost, fmt.Sprintf("/api/translation/items/%d", itemID), nil, nil)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d body=%s", w.Code, w.Body.String())
		}

		item, err := st.GetItem(itemID)
		if err != nil {
			t.Fatalf("reload item: %v", err)
		}
		if item.TranslatedTitle != nil || item.TranslatedContent != nil || item.TranslationUpdatedAt != 0 {
			t.Fatalf("expected no translation to be saved after empty outputs, got %+v", item)
		}
	})
}
