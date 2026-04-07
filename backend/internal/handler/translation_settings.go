package handler

import (
	"errors"
	"strings"

	"github.com/0x2E/fusion/internal/model"
	"github.com/0x2E/fusion/internal/store"
	"github.com/gin-gonic/gin"
)

type translationSettingsRequest struct {
	OpenAIAPIKey              *string `json:"openai_api_key"`
	TranslationModel          *string `json:"translation_model"`
	TranslationTargetLanguage *string `json:"translation_target_language"`
	AutoTranslateMode         *bool   `json:"auto_translate_mode"`
}

type translationSettingsResponse struct {
	HasAPIKey                 bool   `json:"has_api_key"`
	MaskedAPIKey              string `json:"masked_api_key"`
	APIKeySource              string `json:"api_key_source,omitempty"`
	TranslationModel          string `json:"translation_model"`
	TranslationTargetLanguage string `json:"translation_target_language"`
	AutoTranslateMode         bool   `json:"auto_translate_mode"`
}

func (h *Handler) getTranslationSettings(c *gin.Context) {
	settings, err := h.store.GetTranslationSettings()
	if err != nil {
		internalError(c, err, "get translation settings")
		return
	}

	dataResponse(c, h.buildTranslationSettingsResponse(settings))
}

func (h *Handler) patchTranslationSettings(c *gin.Context) {
	var req translationSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequestError(c, "invalid request")
		return
	}

	settings, err := h.store.UpdateTranslationSettings(store.UpdateTranslationSettingsParams{
		OpenAIAPIKey:              req.OpenAIAPIKey,
		TranslationModel:          req.TranslationModel,
		TranslationTargetLanguage: req.TranslationTargetLanguage,
		AutoTranslateMode:         req.AutoTranslateMode,
	})
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			badRequestError(c, "invalid request")
			return
		}
		internalError(c, err, "patch translation settings")
		return
	}

	dataResponse(c, h.buildTranslationSettingsResponse(settings))
}

func (h *Handler) buildTranslationSettingsResponse(settings *model.TranslationSettings) translationSettingsResponse {
	resolvedKey, source := h.resolveTranslationAPIKey(settings.OpenAIAPIKey)

	return translationSettingsResponse{
		HasAPIKey:                 resolvedKey != "",
		MaskedAPIKey:              maskAPIKey(resolvedKey),
		APIKeySource:              source,
		TranslationModel:          settings.TranslationModel,
		TranslationTargetLanguage: settings.TranslationTargetLanguage,
		AutoTranslateMode:         settings.AutoTranslateMode,
	}
}

func (h *Handler) resolveTranslationAPIKey(dbKey string) (string, string) {
	if h != nil && h.config != nil {
		envKey := strings.TrimSpace(h.config.OpenAIAPIKey)
		if envKey != "" {
			return envKey, "env"
		}
	}

	dbKey = strings.TrimSpace(dbKey)
	if dbKey != "" {
		return dbKey, "db"
	}

	return "", ""
}

func maskAPIKey(key string) string {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return ""
	}

	if len(trimmed) <= 8 {
		return strings.Repeat("*", len(trimmed))
	}

	return trimmed[:4] + strings.Repeat("*", len(trimmed)-8) + trimmed[len(trimmed)-4:]
}
