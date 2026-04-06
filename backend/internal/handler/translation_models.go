package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/0x2E/fusion/internal/config"
	"github.com/0x2E/fusion/internal/model"
	"github.com/0x2E/fusion/internal/pkg/httpc"
	"github.com/gin-gonic/gin"
)

const openAIModelsURL = "https://api.openai.com/v1/models"

var errTranslationModelsUnavailable = errors.New("translation models unavailable")

type translationModelsFetcher interface {
	Fetch(ctx context.Context, apiKey string) ([]model.OpenAIModel, error)
}

type openAITranslationModelsFetcher struct {
	client *http.Client
}

type translationModelsResponse struct {
	Models []model.OpenAIModel `json:"models"`
}

type openAIModelsEnvelope struct {
	Data []model.OpenAIModel `json:"data"`
}

func newOpenAITranslationModelsFetcher(cfg *config.Config) translationModelsFetcher {
	allowPrivateFeeds := cfg != nil && cfg.AllowPrivateFeeds
	client, err := httpc.NewClient(30*time.Second, "", allowPrivateFeeds)
	if err != nil {
		return &openAITranslationModelsFetcher{client: http.DefaultClient}
	}

	return &openAITranslationModelsFetcher{client: client}
}

func (h *Handler) getTranslationModels(c *gin.Context) {
	settings, err := h.store.GetTranslationSettings()
	if err != nil {
		internalError(c, err, "get translation settings for models")
		return
	}

	apiKey, _ := h.resolveTranslationAPIKey(settings.OpenAIAPIKey)
	if apiKey == "" {
		dataResponse(c, translationModelsResponse{Models: []model.OpenAIModel{}})
		return
	}

	fetcher := h.translationModelsFetcher
	if fetcher == nil {
		fetcher = newOpenAITranslationModelsFetcher(h.config)
	}

	models, err := fetcher.Fetch(c.Request.Context(), apiKey)
	if err != nil {
		if errors.Is(err, errTranslationModelsUnavailable) {
			dataResponse(c, translationModelsResponse{Models: []model.OpenAIModel{}})
			return
		}

		internalError(c, err, "fetch translation models")
		return
	}

	dataResponse(c, translationModelsResponse{Models: models})
}

func (f *openAITranslationModelsFetcher) Fetch(ctx context.Context, apiKey string) ([]model.OpenAIModel, error) {
	if f == nil || f.client == nil {
		return nil, fmt.Errorf("fetch translation models: %w", errTranslationModelsUnavailable)
	}

	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("fetch translation models: %w", errTranslationModelsUnavailable)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, openAIModelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build translation models request: %w", err)
	}

	httpc.SetDefaultHeaders(req)
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch translation models: %w", errTranslationModelsUnavailable)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch translation models: %w", errTranslationModelsUnavailable)
	}

	var payload openAIModelsEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode translation models response: %w", errTranslationModelsUnavailable)
	}

	return payload.Data, nil
}
