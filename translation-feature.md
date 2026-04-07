# LLM Translation Feature Implementation Plan

## Goal
Implement a high-quality LLM-based translation feature (Russian focus, using OpenAI API) for articles in the Fusion RSS reader. This plan incorporates advanced guardrails for a safe, self-hosted feature.

## Scope Boundaries
- **IN**: Backend endpoint to proxy OpenAI calls, prioritizing environment variables for API keys.
- **IN**: Explicit validation of translated HTML before persistence.
- **IN**: Clear env-overrides-DB behavior for API key resolution.
- **IN**: Database schema updates: create `ai_translation_settings` table. Add `translated_title`, `translated_content`, `translation_model`, `translation_target_language`, `translation_updated_at` to `items`.
- **IN**: Frontend UI updates: a new minimal "AI Translation" section in Settings. In the article viewer, explicit translation states and local UI toggle.
- **OUT**: Non-OpenAI providers (for now).
- **OUT**: Auto-translation on feed pull.

## Architectural Decisions & Guardrails

- **DB Schema**:
  - `ai_translation_settings` table (id, openai_api_key, translation_model, translation_target_language).
  - `items` table adds: `translated_title`, `translated_content`, `translation_model`, `translation_target_language`, `translation_updated_at`.

- **API Key Precedence & PATCH Semantics**:
  - Precedence is strictly **ENV > DB > None**.
  - `GET /api/translation/settings` NEVER returns the key. It returns `has_api_key`, `masked_api_key`, and `api_key_source` ("env" or "db").
  - `PATCH /api/translation/settings` allows partial updates. Empty string for `openai_api_key` clears the DB key. If key comes from ENV, PATCHing the key only updates the DB fallback, not the runtime ENV.

- **Translation Semantics & HTML Guardrail**:
  - Normal `POST /api/translation/items/:id` returns cached translation if one already exists for the same `translation_target_language` and `translation_model`.
  - `?force=true` bypasses cache and performs a new translation. If model or target language changed, treat cache as invalid and perform a new translation.
  - Translate `title` as plain text.
  - If `content` is empty, invalid, or not an HTML fragment, successfully translate `title` and leave `translated_content` as NULL. Do not hard-fail.
  - For valid HTML `content`, backend MUST build a DOM and translate ONLY text nodes (ignoring `script`, `style`, `code`, `pre`).
  - Post-validation: Resulting HTML must parse without errors. Tag node count and order must match exactly. Attributes must not change.
  - A translation is valid only if at least one of `translated_title` or `translated_content` is successfully produced.
  - If validation fails, or both translated fields are empty, endpoint returns an error and does NOT save.
  - `translation_updated_at` updates only after a new translation is successfully persisted.

- **Logging**:
  - Never log the full article body or API key.
  - Log only technical errors.

- **Endpoint Naming**:
  - Use `POST /api/translation/items/:id` for symmetry.

- **Models Endpoint**:
  - `GET /api/translation/models` must degrade gracefully. If key missing or API unreachable, return empty list or 400 error to avoid blocking UI.
  - Settings UI uses a combo box (dropdown + manual text input) with an optional "Fetch models" button. Fallback to manual input.

- **Settings UI**:
  - If `api_key_source` is `env`, the API key input is read-only with a "configured by environment" hint, or provides a clear override UX.

- **Article UI**:
  - State is local to the drawer/article view. No optimistic UI.
  - "Translate" action -> loading spinner + disabled state.
  - Error -> inline toast / small error text.
  - "Show translated / Show original" toggle appears if *either* `translated_title` OR `translated_content` exists (handles mixed state gracefully).
  - "Re-translate" secondary action. Previous translation is the fallback until re-translate succeeds.

---

## Task Dependency Graph

| Task | Depends On | Reason |
|---|---|---|
| Task 1 | None | Foundation for settings and translation data |
| Task 2 | Task 1 | Needs store methods to save/load settings |
| Task 3 | Task 1 | Needs store methods to save/load translations |
| Task 4 | Task 2 | Needs backend API to fetch/save settings and models |
| Task 5 | Task 3, Task 4 | Needs backend API to trigger translation and settings to configure |

## Parallel Execution Graph

Wave 1 (Start immediately):
└── Task 1: Database Migrations & Store Methods

Wave 2 (After Wave 1 completes):
├── Task 2: Backend Settings API & OpenAI Model Fetching
└── Task 3: Backend Translation API

Wave 3 (After Wave 2 completes):
├── Task 4: Frontend Settings UI
└── Task 5: Frontend Article Translation UI

## Category + Skills Recommendations

| Category | Skills | Justification |
|---|---|---|
| `unspecified-high` | `git-master` | Backend tasks require complex Go/SQL logic and atomic commits. |
| `visual-engineering` | `frontend-ui-ux`, `git-master` | Frontend tasks require React/Tailwind/shadcn expertise and atomic commits. |

---

## Actionable TODO List for Caller

> CALLER: Add these TODOs using TodoWrite/TaskCreate and execute by wave.

### Wave 1

- [ ] **1. Database Migrations & Store Methods**
  - What: Create `004_ai_translation_settings.sql` migration to create `ai_translation_settings`. Add `translated_title`, `translated_content`, `translation_model`, `translation_target_language`, `translation_updated_at` to `items` table. Update `store.Item` struct and store methods.
  - Depends: None
  - Category: `unspecified-high`
  - Skills: `git-master`
  - QA: Run `sqlite3 fusion.db ".schema ai_translation_settings"` to verify table creation. Verify `items` table schema. Run `cd backend && go test ./internal/store/...` to verify store logic.

### Wave 2

- [ ] **2. Backend Settings API & OpenAI Model Fetching**
  - What: Create `GET /api/translation/settings`, `PATCH /api/translation/settings`, and `GET /api/translation/models` endpoints. `GET /api/translation/settings` returns `has_api_key`, `masked_api_key`, `api_key_source` ("env" or "db"), `model`, `target_language`. `GET /api/translation/models` degrades gracefully. Support env override for API key.
  - Depends: Task 1
  - Category: `unspecified-high`
  - Skills: `git-master`
  - QA: Test `PATCH` endpoint. Verify `GET` returns correct `api_key_source` and masked key. Test `/api/translation/models` graceful degradation.

- [ ] **3. Backend Translation API (with HTML Guardrails)**
  - What: Create `POST /api/translation/items/:id` endpoint. Normal `POST` returns cached translation for the same `translation_target_language` and `translation_model`; `?force=true` bypasses cache. If model or target language changed, cache is invalid and a new translation runs. Fetch item, translate `title` as plain text, translate `content` by building a DOM and translating only text nodes (ignoring script/style/code/pre). If `content` is empty/invalid, save title-only translation and leave `translated_content` as NULL. Post-validation: parse HTML without errors, tag count/order matches, attributes intact. Treat translation as valid only if at least one of `translated_title` or `translated_content` is produced. Save `translated_title`, `translated_content`, `translation_model`, `translation_target_language`, `translation_updated_at` only after successful validation and persistence. No full body or API key in logs.
  - Depends: Task 1
  - Category: `unspecified-high`
  - Skills: `git-master`
  - QA: Test valid/invalid HTML inputs, verify DOM structure preservation. Verify same model/language returns cache, changed model/language invalidates cache, `?force=true` bypasses cache, and empty model output is rejected. Check logs for absence of sensitive data.

### Wave 3

- [ ] **4. Frontend Settings UI**
  - What: Add minimal "AI Translation" section to `SettingsDialog`. Show `has_api_key` + masked key. If `api_key_source` is `env`, show "configured by environment" read-only hint. Implement combo box for model selection with optional "Fetch models" button. Target language defaults to "ru". Do not return source key to UI.
  - Depends: Task 2
  - Category: `visual-engineering`
  - Skills: `frontend-ui-ux`, `git-master`, `dev-browser`
  - QA: Use `dev-browser` to test UI. Verify combo box, read-only input for env, and fetch button functionality.

- [ ] **5. Frontend Article Translation UI**
  - What: Add "Translate" button to `ArticleItem`/`ArticleDrawer`. If no translation -> "Translate". If translation exists -> "Show translated / Show original" toggle + secondary "Re-translate" action. Use local state. Loading spinner + disabled state. Inline toast on error. Re-translate retains old translation as fallback.
  - Depends: Task 3, Task 4
  - Category: `visual-engineering`
  - Skills: `frontend-ui-ux`, `git-master`, `dev-browser`
  - QA: Use `dev-browser` to test article translation flow. Verify loading, toggle, error states, and re-translate fallback.