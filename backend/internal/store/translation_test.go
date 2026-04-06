package store

import (
	"errors"
	"testing"

	"github.com/0x2E/fusion/internal/model"
)

func TestTranslationSettingsStore(t *testing.T) {
	store, _ := setupTestDB(t)
	defer closeStore(t, store)

	t.Run("returns defaults when settings are unset", func(t *testing.T) {
		settings, err := store.GetTranslationSettings()
		if err != nil {
			t.Fatalf("GetTranslationSettings() failed: %v", err)
		}

		if settings.OpenAIAPIKey != "" {
			t.Errorf("expected empty api key, got %q", settings.OpenAIAPIKey)
		}
		if settings.TranslationModel != "" {
			t.Errorf("expected empty translation model, got %q", settings.TranslationModel)
		}
		if settings.TranslationTargetLanguage != "" {
			t.Errorf("expected empty target language, got %q", settings.TranslationTargetLanguage)
		}
	})

	t.Run("persists settings and supports partial updates with key clearing", func(t *testing.T) {
		apiKey := "sk-test-secret"
		translationModel := "gpt-4o-mini"
		targetLanguage := "ru"

		settings, err := store.UpdateTranslationSettings(UpdateTranslationSettingsParams{
			OpenAIAPIKey:              &apiKey,
			TranslationModel:          &translationModel,
			TranslationTargetLanguage: &targetLanguage,
		})
		if err != nil {
			t.Fatalf("UpdateTranslationSettings() failed: %v", err)
		}

		assertTranslationSettings(t, settings, model.TranslationSettings{
			OpenAIAPIKey:              apiKey,
			TranslationModel:          translationModel,
			TranslationTargetLanguage: targetLanguage,
		})

		reloaded, err := store.GetTranslationSettings()
		if err != nil {
			t.Fatalf("GetTranslationSettings() failed: %v", err)
		}
		assertTranslationSettings(t, reloaded, model.TranslationSettings{
			OpenAIAPIKey:              apiKey,
			TranslationModel:          translationModel,
			TranslationTargetLanguage: targetLanguage,
		})

		updatedLanguage := "de"
		clearedKey := ""
		updated, err := store.UpdateTranslationSettings(UpdateTranslationSettingsParams{
			OpenAIAPIKey:              &clearedKey,
			TranslationTargetLanguage: &updatedLanguage,
		})
		if err != nil {
			t.Fatalf("UpdateTranslationSettings() partial update failed: %v", err)
		}

		assertTranslationSettings(t, updated, model.TranslationSettings{
			OpenAIAPIKey:              "",
			TranslationModel:          translationModel,
			TranslationTargetLanguage: updatedLanguage,
		})

		reloaded, err = store.GetTranslationSettings()
		if err != nil {
			t.Fatalf("GetTranslationSettings() after partial update failed: %v", err)
		}
		assertTranslationSettings(t, reloaded, model.TranslationSettings{
			OpenAIAPIKey:              "",
			TranslationModel:          translationModel,
			TranslationTargetLanguage: updatedLanguage,
		})
	})
}

func TestItemTranslationStore(t *testing.T) {
	store, _ := setupTestDB(t)
	defer closeStore(t, store)

	group := mustCreateGroup(t, store, "Translation Group")
	feed := mustCreateFeed(t, store, group.ID, "Translation Feed", "https://example.com/translation.xml", "https://example.com", "")
	item := mustCreateItem(t, store, feed.ID, "translation-guid", "Original title", "https://example.com/item", "<p>Original content</p>", 123)

	t.Run("returns empty cache metadata when no translation exists", func(t *testing.T) {
		cache, err := store.GetItemTranslationCache(item.ID)
		if err != nil {
			t.Fatalf("GetItemTranslationCache() failed: %v", err)
		}

		if cache.ItemID != item.ID {
			t.Fatalf("expected cache item id %d, got %d", item.ID, cache.ItemID)
		}
		if cache.TranslatedTitle != nil {
			t.Errorf("expected nil translated title, got %q", *cache.TranslatedTitle)
		}
		if cache.TranslatedContent != nil {
			t.Errorf("expected nil translated content, got %q", *cache.TranslatedContent)
		}
		if cache.TranslationModel != "" {
			t.Errorf("expected empty translation model, got %q", cache.TranslationModel)
		}
		if cache.TranslationTargetLanguage != "" {
			t.Errorf("expected empty target language, got %q", cache.TranslationTargetLanguage)
		}
		if cache.TranslationUpdatedAt != 0 {
			t.Errorf("expected zero translation_updated_at, got %d", cache.TranslationUpdatedAt)
		}
	})

	t.Run("persists cache metadata and item translation fields", func(t *testing.T) {
		translatedTitle := "Переведенный заголовок"
		translatedContent := "<p>Переведенный контент</p>"
		updatedAt := int64(1710000000)

		err := store.SaveItemTranslation(item.ID, SaveItemTranslationInput{
			TranslatedTitle:           &translatedTitle,
			TranslatedContent:         &translatedContent,
			TranslationModel:          "gpt-4o-mini",
			TranslationTargetLanguage: "ru",
			TranslationUpdatedAt:      updatedAt,
		})
		if err != nil {
			t.Fatalf("SaveItemTranslation() failed: %v", err)
		}

		cache, err := store.GetItemTranslationCache(item.ID)
		if err != nil {
			t.Fatalf("GetItemTranslationCache() failed: %v", err)
		}

		assertStringPointerValue(t, cache.TranslatedTitle, translatedTitle, "cache translated title")
		assertStringPointerValue(t, cache.TranslatedContent, translatedContent, "cache translated content")
		if cache.TranslationModel != "gpt-4o-mini" {
			t.Errorf("expected translation model gpt-4o-mini, got %q", cache.TranslationModel)
		}
		if cache.TranslationTargetLanguage != "ru" {
			t.Errorf("expected target language ru, got %q", cache.TranslationTargetLanguage)
		}
		if cache.TranslationUpdatedAt != updatedAt {
			t.Errorf("expected translation_updated_at %d, got %d", updatedAt, cache.TranslationUpdatedAt)
		}

		reloaded, err := store.GetItem(item.ID)
		if err != nil {
			t.Fatalf("GetItem() failed: %v", err)
		}

		assertStringPointerValue(t, reloaded.TranslatedTitle, translatedTitle, "item translated title")
		assertStringPointerValue(t, reloaded.TranslatedContent, translatedContent, "item translated content")
		if reloaded.TranslationModel != "gpt-4o-mini" {
			t.Errorf("expected item translation model gpt-4o-mini, got %q", reloaded.TranslationModel)
		}
		if reloaded.TranslationTargetLanguage != "ru" {
			t.Errorf("expected item target language ru, got %q", reloaded.TranslationTargetLanguage)
		}
		if reloaded.TranslationUpdatedAt != updatedAt {
			t.Errorf("expected item translation_updated_at %d, got %d", updatedAt, reloaded.TranslationUpdatedAt)
		}
	})

	t.Run("allows title-only translation and rejects empty saves without updating timestamp", func(t *testing.T) {
		titleOnly := "Nur Titel"
		titleOnlyUpdatedAt := int64(1710000100)

		err := store.SaveItemTranslation(item.ID, SaveItemTranslationInput{
			TranslatedTitle:           &titleOnly,
			TranslationModel:          "gpt-4.1-mini",
			TranslationTargetLanguage: "de",
			TranslationUpdatedAt:      titleOnlyUpdatedAt,
		})
		if err != nil {
			t.Fatalf("SaveItemTranslation() title-only failed: %v", err)
		}

		cache, err := store.GetItemTranslationCache(item.ID)
		if err != nil {
			t.Fatalf("GetItemTranslationCache() after title-only save failed: %v", err)
		}
		assertStringPointerValue(t, cache.TranslatedTitle, titleOnly, "title-only cache translated title")
		if cache.TranslatedContent != nil {
			t.Errorf("expected nil translated content after title-only save, got %q", *cache.TranslatedContent)
		}
		if cache.TranslationModel != "gpt-4.1-mini" {
			t.Errorf("expected updated translation model, got %q", cache.TranslationModel)
		}
		if cache.TranslationTargetLanguage != "de" {
			t.Errorf("expected updated target language, got %q", cache.TranslationTargetLanguage)
		}
		if cache.TranslationUpdatedAt != titleOnlyUpdatedAt {
			t.Errorf("expected updated translation_updated_at %d, got %d", titleOnlyUpdatedAt, cache.TranslationUpdatedAt)
		}

		err = store.SaveItemTranslation(item.ID, SaveItemTranslationInput{
			TranslationModel:          "gpt-4.1-mini",
			TranslationTargetLanguage: "de",
			TranslationUpdatedAt:      titleOnlyUpdatedAt + 10,
		})
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("expected ErrInvalid for empty translation save, got %v", err)
		}

		cacheAfterInvalid, err := store.GetItemTranslationCache(item.ID)
		if err != nil {
			t.Fatalf("GetItemTranslationCache() after invalid save failed: %v", err)
		}
		if cacheAfterInvalid.TranslationUpdatedAt != titleOnlyUpdatedAt {
			t.Errorf("expected translation_updated_at to remain %d after invalid save, got %d", titleOnlyUpdatedAt, cacheAfterInvalid.TranslationUpdatedAt)
		}
		assertStringPointerValue(t, cacheAfterInvalid.TranslatedTitle, titleOnly, "title-only cache translated title after invalid save")
		if cacheAfterInvalid.TranslatedContent != nil {
			t.Errorf("expected nil translated content after invalid save, got %q", *cacheAfterInvalid.TranslatedContent)
		}
	})
}

func assertTranslationSettings(t *testing.T, got *model.TranslationSettings, want model.TranslationSettings) {
	t.Helper()

	if got == nil {
		t.Fatal("expected translation settings, got nil")
	}
	if got.OpenAIAPIKey != want.OpenAIAPIKey {
		t.Errorf("expected api key %q, got %q", want.OpenAIAPIKey, got.OpenAIAPIKey)
	}
	if got.TranslationModel != want.TranslationModel {
		t.Errorf("expected translation model %q, got %q", want.TranslationModel, got.TranslationModel)
	}
	if got.TranslationTargetLanguage != want.TranslationTargetLanguage {
		t.Errorf("expected target language %q, got %q", want.TranslationTargetLanguage, got.TranslationTargetLanguage)
	}
}

func assertStringPointerValue(t *testing.T, got *string, want string, field string) {
	t.Helper()

	if got == nil {
		t.Fatalf("expected %s to be set", field)
	}
	if *got != want {
		t.Errorf("expected %s %q, got %q", field, want, *got)
	}
}
