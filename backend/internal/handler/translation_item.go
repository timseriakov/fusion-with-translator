package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/0x2E/fusion/internal/model"

	"github.com/0x2E/fusion/internal/config"
	"github.com/0x2E/fusion/internal/pkg/httpc"
	"github.com/0x2E/fusion/internal/store"
	"github.com/gin-gonic/gin"
)

const openAIChatURL = "https://api.openai.com/v1/chat/completions"

var errTranslationUnavailable = errors.New("translation unavailable")

type itemTranslator interface {
	Translate(ctx context.Context, apiKey, model, systemPrompt, userPrompt string) (string, error)
}

type openAIItemTranslator struct {
	client *http.Client
}

func newItemTranslator(cfg *config.Config) itemTranslator {
	allowPrivateFeeds := cfg != nil && cfg.AllowPrivateFeeds
	client, err := httpc.NewClient(120*time.Second, "", allowPrivateFeeds)
	if err != nil {
		return &openAIItemTranslator{client: http.DefaultClient}
	}
	return &openAIItemTranslator{client: client}
}

type openAIChatRequest struct {
	Model     string              `json:"model"`
	Messages  []openAIChatMessage `json:"messages"`
	MaxTokens int                 `json:"max_tokens,omitempty"`
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatResponse struct {
	Choices []openAIChatChoice `json:"choices"`
}

type openAIChatChoice struct {
	Message openAIChatMessage `json:"message"`
}

func (t *openAIItemTranslator) Translate(ctx context.Context, apiKey, model, systemPrompt, userPrompt string) (string, error) {
	if t == nil || t.client == nil {
		return "", fmt.Errorf("translate: %w", errTranslationUnavailable)
	}

	payload := openAIChatRequest{
		Model: model,
		Messages: []openAIChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens: 4096,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal chat request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIChatURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build chat request: %w", err)
	}

	httpc.SetDefaultHeaders(req)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("chat request: %w", errTranslationUnavailable)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("chat request status %d: %w", resp.StatusCode, errTranslationUnavailable)
	}

	var decoded openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", fmt.Errorf("decode chat response: %w", errTranslationUnavailable)
	}
	if len(decoded.Choices) == 0 {
		return "", fmt.Errorf("empty chat choices: %w", errTranslationUnavailable)
	}

	return strings.TrimSpace(decoded.Choices[0].Message.Content), nil
}

func (h *Handler) translateItem(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		badRequestError(c, "invalid id")
		return
	}

	force := false
	if rawForce := strings.TrimSpace(c.Query("force")); rawForce != "" {
		force, err = strconv.ParseBool(rawForce)
		if err != nil {
			badRequestError(c, "invalid force")
			return
		}
	}

	settings, err := h.store.GetTranslationSettings()
	if err != nil {
		internalError(c, err, "get translation settings for item")
		return
	}

	apiKey, _ := h.resolveTranslationAPIKey(settings.OpenAIAPIKey)
	if apiKey == "" {
		badRequestError(c, "no API key configured")
		return
	}
	if strings.TrimSpace(settings.TranslationModel) == "" {
		badRequestError(c, "no translation model configured")
		return
	}
	if strings.TrimSpace(settings.TranslationTargetLanguage) == "" {
		badRequestError(c, "no target language configured")
		return
	}

	if err := h.doTranslateItem(c.Request.Context(), id, settings, apiKey, force); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			notFoundError(c, "item")
			return
		}
		if errors.Is(err, errTranslationProducedNoOutput) {
			badRequestError(c, err.Error())
			return
		}
		internalError(c, err, "translate item")
		return
	}

	updatedItem, err := h.store.GetItem(id)
	if err != nil {
		internalError(c, err, "get translated item")
		return
	}

	dataResponse(c, updatedItem)
}

var errTranslationProducedNoOutput = errors.New("translation produced no output")

func (h *Handler) doTranslateItem(ctx context.Context, id int64, settings *model.TranslationSettings, apiKey string, force bool) error {
	item, err := h.store.GetItem(id)
	if err != nil {
		return err
	}

	if !force {
		cache, err := h.store.GetItemTranslationCache(id)
		if err == nil && cache.TranslationModel == settings.TranslationModel && cache.TranslationTargetLanguage == settings.TranslationTargetLanguage && cache.TranslationUpdatedAt > 0 && (cache.TranslatedTitle != nil || cache.TranslatedContent != nil) {
			contentNeedsTranslation := strings.TrimSpace(item.Content) != ""
			if !contentNeedsTranslation || cache.TranslatedContent != nil {
				return nil
			}
		}
	}

	translator := h.itemTranslator
	if translator == nil {
		translator = newItemTranslator(h.config)
	}

	var translatedTitle *string
	if strings.TrimSpace(item.Title) != "" {
		translated, err := translator.Translate(
			ctx,
			apiKey,
			settings.TranslationModel,
			fmt.Sprintf("Translate the following plain text to %s. Return only the translated text.", settings.TranslationTargetLanguage),
			item.Title,
		)
		if err != nil {
			return fmt.Errorf("translate item title: %w", err)
		}
		translated = strings.TrimSpace(translated)
		if translated != "" {
			translatedTitle = &translated
		}
	}

	var translatedContent *string
	var translatedExcerpt *string
	if trimmedContent := strings.TrimSpace(item.Content); trimmedContent != "" {
		translated, err := translator.Translate(
			ctx,
			apiKey,
			settings.TranslationModel,
			fmt.Sprintf("Translate the following Markdown text to %s. Preserve all Markdown formatting, links, URLs, and code blocks exactly as they are. Only translate the natural language text. Return only the translated Markdown.", settings.TranslationTargetLanguage),
			trimmedContent,
		)
		if err != nil {
			return fmt.Errorf("translate item content: %w", err)
		}
		translated = strings.TrimSpace(translated)
		if translated != "" {
			translatedContent = &translated
			excerpt := extractPlainTextExcerpt(translated, 300)
			if excerpt != "" {
				translatedExcerpt = &excerpt
			}
		}
	}

	if translatedTitle == nil && translatedContent == nil && translatedExcerpt == nil {
		return errTranslationProducedNoOutput
	}

	if err := h.store.SaveItemTranslation(id, store.SaveItemTranslationInput{
		TranslatedTitle:           translatedTitle,
		TranslatedContent:         translatedContent,
		TranslatedExcerpt:         translatedExcerpt,
		TranslationModel:          settings.TranslationModel,
		TranslationTargetLanguage: settings.TranslationTargetLanguage,
		TranslationUpdatedAt:      time.Now().Unix(),
	}); err != nil {
		if errors.Is(err, store.ErrInvalid) {
			return errTranslationProducedNoOutput
		}
		return err
	}

	return nil
}

func (h *Handler) translateItemsBatch(c *gin.Context) {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequestError(c, "invalid request body")
		return
	}

	if len(req.IDs) == 0 {
		badRequestError(c, "no ids provided")
		return
	}
	if len(req.IDs) > 20 {
		badRequestError(c, "batch size exceeds limit of 20")
		return
	}

	settings, err := h.store.GetTranslationSettings()
	if err != nil {
		internalError(c, err, "get translation settings for batch")
		return
	}

	apiKey, _ := h.resolveTranslationAPIKey(settings.OpenAIAPIKey)
	if apiKey == "" {
		badRequestError(c, "no API key configured")
		return
	}
	if strings.TrimSpace(settings.TranslationModel) == "" {
		badRequestError(c, "no translation model configured")
		return
	}
	if strings.TrimSpace(settings.TranslationTargetLanguage) == "" {
		badRequestError(c, "no target language configured")
		return
	}

	type result struct {
		translated []int64
		failed     []int64
		errors     map[string]string
	}
	res := result{
		translated: make([]int64, 0),
		failed:     make([]int64, 0),
		errors:     make(map[string]string),
	}
	var mu sync.Mutex

	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup

	for _, id := range req.IDs {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			err := h.doTranslateItem(c.Request.Context(), id, settings, apiKey, false)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				res.failed = append(res.failed, id)
				res.errors[strconv.FormatInt(id, 10)] = err.Error()
			} else {
				res.translated = append(res.translated, id)
			}
		}(id)
	}

	wg.Wait()

	dataResponse(c, gin.H{
		"translated": res.translated,
		"failed":     res.failed,
		"errors":     res.errors,
	})
}

var (
	reImage      = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	reLink       = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	reHeading    = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	reBold       = regexp.MustCompile(`\*\*([^*]*)\*\*`)
	reFencedCode = regexp.MustCompile("(?s)```[^`]*```")
	reInlineCode = regexp.MustCompile("`([^`]*)`")
)

// extractPlainTextExcerpt strips Markdown formatting and returns a plain text excerpt of maxLen runes.
func extractPlainTextExcerpt(markdownContent string, maxLen int) string {
	if markdownContent == "" || maxLen <= 0 {
		return ""
	}

	text := reImage.ReplaceAllString(markdownContent, "")
	text = reLink.ReplaceAllString(text, "$1")
	text = reHeading.ReplaceAllString(text, "")
	text = reBold.ReplaceAllString(text, "$1")
	text = reFencedCode.ReplaceAllString(text, "")
	text = reInlineCode.ReplaceAllString(text, "$1")

	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}
	return string(runes[:maxLen])
}
