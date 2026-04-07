package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
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
	TranslatedExcerpt         *string `json:"translated_excerpt"`
	TranslationModel          string  `json:"translation_model"`
	TranslationTargetLanguage string  `json:"translation_target_language"`
	TranslationUpdatedAt      int64   `json:"translation_updated_at"`
}

type stubItemTranslator struct {
	mu        sync.Mutex
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
	s.mu.Lock()
	defer s.mu.Unlock()

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

	group, err := st.CreateGroup(fmt.Sprintf("Translation Group %s", title))
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	feed, err := st.CreateFeed(group.ID, fmt.Sprintf("Translation Feed %s", title), fmt.Sprintf("https://example.com/feed-%s.xml", title), "https://example.com", "")
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
		cachedExcerpt := "Привет мир"
		if err := st.SaveItemTranslation(itemID, store.SaveItemTranslationInput{
			TranslatedTitle:           &cachedTitle,
			TranslatedContent:         &cachedContent,
			TranslatedExcerpt:         &cachedExcerpt,
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
		if response.Data.TranslatedExcerpt == nil || *response.Data.TranslatedExcerpt != "Привет мир" {
			t.Fatalf("expected cached translated excerpt \"Привет мир\", got %#v", response.Data.TranslatedExcerpt)
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
		if response.Data.TranslatedExcerpt == nil || *response.Data.TranslatedExcerpt == "" {
			t.Fatal("expected refreshed translated excerpt to be present")
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
		if response.Data.TranslatedExcerpt != nil {
			t.Fatalf("expected translated excerpt to stay nil, got %#v", response.Data.TranslatedExcerpt)
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
		if response.Data.TranslatedExcerpt == nil || *response.Data.TranslatedExcerpt != "Просто обычный текст, без HTML тегов." {
			t.Fatalf("expected translated plain text excerpt, got %#v", response.Data.TranslatedExcerpt)
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

	t.Run("translates html with unclosed p tags (HN-style)", func(t *testing.T) {
		h, st := newTranslationItemTestHandler(t, &config.Config{})
		mustSeedTranslationSettings(t, st, "sk-db-secret", "gpt-4o-mini", "ru")
		// Unclosed <p> tags are valid HTML5 — browsers auto-close them.
		// The parser normalises them: each <p> becomes properly closed.
		content := "<p>Hello all<p>- Item one\n- Item two<p>Closing line</p>"
		itemID := seedTranslationItemFixture(t, st, "Original title", content)

		translator := &stubItemTranslator{responses: []stubTranslationResult{
			{output: "Перевод заголовка"},
			// LLM returns multi-line node [1] as continuation lines (no [N] prefix)
			{output: "[0] Привет всем\n[1] - Пункт один\n- Пункт два\n[2] Заключение"},
		}}
		h.itemTranslator = translator

		w := performRequest(h.SetupRouter(), http.MethodPost, fmt.Sprintf("/api/translation/items/%d", itemID), nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d body=%s", w.Code, w.Body.String())
		}

		response := decodeTranslateItemResponse(t, w.Body.Bytes())
		if response.Data.TranslatedContent == nil {
			t.Fatal("expected translated content to be present")
		}
		if !strings.Contains(*response.Data.TranslatedContent, "Привет всем") {
			t.Fatalf("expected translated content to contain 'Привет всем', got %q", *response.Data.TranslatedContent)
		}
		if !strings.Contains(*response.Data.TranslatedContent, "Пункт один") || !strings.Contains(*response.Data.TranslatedContent, "Пункт два") {
			t.Fatalf("expected multi-line translation preserved, got %q", *response.Data.TranslatedContent)
		}
	})
}

func TestExtractPlainTextExcerpt(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		maxLen   int
		expected string
	}{
		{
			name:     "empty content",
			html:     "",
			maxLen:   10,
			expected: "",
		},
		{
			name:     "no tags",
			html:     "hello world",
			maxLen:   20,
			expected: "hello world",
		},
		{
			name:     "simple tags",
			html:     "<p>hello</p> <strong>world</strong>",
			maxLen:   20,
			expected: "hello world",
		},
		{
			name:     "nested tags",
			html:     "<div><p>hello <span>world</span></p></div>",
			maxLen:   20,
			expected: "hello world",
		},
		{
			name:     "whitespace collapsing",
			html:     "  hello \n\t world  ",
			maxLen:   20,
			expected: "hello world",
		},
		{
			name:     "truncation at rune boundary",
			html:     "hello world",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "multi-byte runes truncation",
			html:     "Привет мир", // "Привет" is 6 runes
			maxLen:   6,
			expected: "Привет",
		},
		{
			name:     "html entities decoding",
			html:     "hello &amp; world",
			maxLen:   20,
			expected: "hello & world",
		},
		{
			name:     "script and style tags (should be stripped)",
			html:     "<script>alert(1)</script>hello<style>body{}</style>",
			maxLen:   20,
			expected: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := extractPlainTextExcerpt(tt.html, tt.maxLen)
			if actual != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}

func TestTranslateItemsBatch(t *testing.T) {
	h, st := newTranslationItemTestHandler(t, &config.Config{})
	mustSeedTranslationSettings(t, st, "sk-db-secret", "gpt-4o-mini", "ru")

	id1 := seedTranslationItemFixture(t, st, "Title 1", "Content 1")
	id2 := seedTranslationItemFixture(t, st, "Title 2", "Content 2")
	id3 := seedTranslationItemFixture(t, st, "Title 3", "Content 3")

	translator := &stubItemTranslator{
		responses: []stubTranslationResult{
			{output: "Translated Title 1"},
			{output: "Translated Content 1"},
			{output: "Translated Title 2"},
			{output: "Translated Content 2"},
			{output: "Translated Title 3"},
			{err: errors.New("translation failed")},
		},
	}
	h.itemTranslator = translator

	body := map[string]any{
		"ids": []int64{id1, id2, id3},
	}
	jsonBody, _ := json.Marshal(body)

	w := performRequest(h.SetupRouter(), http.MethodPost, "/api/translation/items/batch", strings.NewReader(string(jsonBody)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			Translated []int64           `json:"translated"`
			Failed     []int64           `json:"failed"`
			Errors     map[string]string `json:"errors"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Data.Translated) != 2 {
		t.Errorf("expected 2 translated items, got %d", len(resp.Data.Translated))
	}
	if len(resp.Data.Failed) != 1 {
		t.Errorf("expected 1 failed item, got %d", len(resp.Data.Failed))
	}
	if len(resp.Data.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(resp.Data.Errors))
	}
	for idStr, errMsg := range resp.Data.Errors {
		if !strings.Contains(errMsg, "translation failed") {
			t.Errorf("expected error message for item %s to contain 'translation failed', got %q", idStr, errMsg)
		}
	}
}
