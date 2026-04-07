- Migrations are applied automatically on startup in the Fusion backend.
- SQLite uses INTEGER for boolean flags (0/1).
- Migration sequence can have gaps (e.g., 003 was missing, but 005 was added as requested).
- Verification of schema changes can be done using `sqlite3 fusion.db "PRAGMA table_info(table_name)"`.
- Added `AutoTranslateMode` to `TranslationSettings` and `TranslatedExcerpt` to `Item` and `ItemTranslationCache`.
- Pointers (`*string`) are used for optional/nullable string fields in models.
## Store Updates for Translation Fields
- Updated `backend/internal/store/item.go` to handle `translated_excerpt` in `ListItems`, `GetItem`, `ListFeverItems`, and `GetItemTranslationCache`.
- Updated `SaveItemTranslation` to persist `translated_excerpt`.
- Updated `GetTranslationSettings` and `UpdateTranslationSettings` to handle `auto_translate_mode`.
- Used `boolToInt` and `intToBool` helpers for SQLite boolean handling.
- Ensured `UpdateTranslationSettings` supports partial updates using pointers in `UpdateTranslationSettingsParams`.
- Verified with `go build` and `go test` in the `backend` directory.

## Task 5: Update translateItem handler to compute and save translated_excerpt
- Updated `translateItem` handler in `backend/internal/handler/translation_item.go` to compute `translatedExcerpt` using `extractPlainTextExcerpt` after successful translation of content (either HTML or plain text).
- Passed `translatedExcerpt` to `h.store.SaveItemTranslation`.
- Updated `translateItemPayload` in `backend/internal/handler/translation_item_test.go` to include `translated_excerpt`.
- Updated test cases in `backend/internal/handler/translation_item_test.go` to verify that `translated_excerpt` is correctly saved and returned in the API response.
- Verified that `translatedExcerpt` is `nil` if translation produced no content.
## Translation Settings Update
- Added `auto_translate_mode` to translation settings handler.
- Updated request and response structs to include the new field.
- Verified with tests in `translation_settings_test.go`.
- The store already supported `AutoTranslateMode`, so only handler and tests needed updates.

## Auto-Translate Batch Logic
- Implemented batch translation in `ArticleList` using `useEffect` to trigger translations in chunks of 10.
- Tracked `translatingIds` in state to avoid redundant requests and show loading indicators.
- Updated `ArticleItem` to conditionally display translated titles and excerpts when `autoTranslateMode` is enabled.
- Added a small `Loader2` spinner in `ArticleItem` to indicate ongoing translation for specific articles.

## ArticleDrawer Auto-Translate
- Implemented auto-translate on open in `ArticleDrawer`.
- Used two `useEffect` hooks to handle auto-translate and reset logic separately.
- This avoids the "manual toggle bug" where manual translations would be hidden if `auto_translate_mode` was false.
- The auto-translate effect depends on `selectedArticleId`, `auto_translate_mode`, `article?.id`, and `hasTranslation`.
- The reset effect only runs on `selectedArticleId` change and only if `auto_translate_mode` is false.

## Auto-translate Mode Toggle
- Added a `Switch` component to `TranslationSettingsContent` to toggle `auto_translate_mode`.
- The state is initialized from the fetched translation settings.
- The toggle is disabled if an update is pending or if no API key is configured.
- Added i18n keys `settings.translation.autoMode.label` and `settings.translation.autoMode.description` to `en.ts` and `zh.ts`.
- Verified that the `updatePayload` correctly includes `auto_translate_mode` only when it has changed.
