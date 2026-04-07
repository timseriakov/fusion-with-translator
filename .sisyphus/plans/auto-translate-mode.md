# Plan: Auto-Translate Mode

## Goal
Add a persistent "auto-translate mode" toggle to Settings. When enabled:
1. **Item list**: shows translated title + excerpt (from DB) for already-translated items; for un-translated items, shows original with a small spinner while translating in background (in batches).
2. **Article drawer**: auto-translates on open (if no cached translation exists). Force re-translate also regenerates excerpt.

## Scope
**IN:**
- DB migration 005: `auto_translate_mode` in `ai_translation_settings`, `translated_excerpt` in `items`
- Backend: model, store, handler updates; new batch translate endpoint; excerpt computed on translate
- Frontend: item list shows translated title/excerpt + spinner; drawer auto-translates; settings toggle
- i18n for all 8 locales

**OUT:**
- Non-OpenAI providers
- Auto-translate on feed pull (background job)
- Backward compatibility for existing translations without excerpt (they just won't show excerpt until re-translated)

## Success Criteria
- Toggle in Settings persists to DB, is returned by `GET /api/translation/settings`
- Item list: translated title + excerpt displayed when mode=ON and translation cached
- Item list: spinner inline next to date for items being translated in background
- Drawer: auto-translates on open when mode=ON and no cached translation
- Force re-translate regenerates excerpt too
- All existing tests pass; new tests for batch endpoint and excerpt logic
- TypeScript compiles clean; Go builds clean

---

- [x] Task 1 — DB Migration 005
**File:** `backend/internal/store/migrations/005_auto_translate_mode.sql`

```sql
ALTER TABLE ai_translation_settings ADD COLUMN auto_translate_mode INTEGER NOT NULL DEFAULT 0;
ALTER TABLE items ADD COLUMN translated_excerpt TEXT;
```

**QA:** Run app, check migration applies cleanly. Check `PRAGMA table_info(ai_translation_settings)` and `PRAGMA table_info(items)`.

---

- [x] Task 2 — Backend: Model Update
**File:** `backend/internal/model/model.go`

- Add `AutoTranslateMode bool` to `TranslationSettings` struct (json: `"auto_translate_mode"`)
- Add `TranslatedExcerpt *string` to `Item` struct (json: `"translated_excerpt,omitempty"`)
- Add `TranslatedExcerpt *string` to `ItemTranslationCache` struct

**QA:** `go build -o /dev/null ./...` passes.

---

- [x] Task 3 — Backend: Store Updates
**Files:** `backend/internal/store/item.go`, `backend/internal/store/translation_settings.go` (or wherever `GetTranslationSettings`/`UpdateTranslationSettings` live)

### 3a. `ListItems` SQL query
Extend SELECT to include `items.translated_excerpt`. Add `&item.TranslatedExcerpt` to scan.

### 3b. `GetItem` SQL query
Same — add `translated_excerpt` to SELECT and scan.

### 3c. `SaveItemTranslation`
- Add `TranslatedExcerpt *string` to `SaveItemTranslationInput` struct
- Add `translated_excerpt = :translated_excerpt` to UPDATE SQL (or INSERT OR REPLACE)

### 3d. `GetTranslationSettings` / `UpdateTranslationSettings`
- Include `auto_translate_mode` in SELECT and scan
- `UpdateTranslationSettingsParams`: add `AutoTranslateMode *bool` (pointer = optional patch)
- UPDATE SQL: if `AutoTranslateMode != nil`, set `auto_translate_mode = :auto_translate_mode`

**QA:** `go test -count=1 ./internal/store/...` passes.

---

- [x] Task 4 — Backend: Excerpt Computation Helper
**File:** `backend/internal/handler/translation_item.go`

Add function `extractPlainTextExcerpt(html string, maxLen int) string`:
- Strip HTML tags (use `html.NewTokenizer` or regex-free approach: collect text tokens)
- Trim whitespace
- Truncate to `maxLen` runes (not bytes)
- Use `maxLen = 300` (generous — frontend can trim display to 150 chars)

**QA:** Unit test in `translation_item_test.go`:
```go
t.Run("extractPlainTextExcerpt strips HTML and truncates", func(t *testing.T) {
    input := "<p>Hello <strong>world</strong>, this is a test.</p>"
    got := extractPlainTextExcerpt(input, 20)
    want := "Hello world, this is"
    if got != want { t.Fatalf("got %q want %q", got, want) }
})
```

---

- [x] Task 5 — Backend: Wire Excerpt into Translation Handler
**File:** `backend/internal/handler/translation_item.go`

In `translateItem` handler, after computing `translatedContent`:
```go
var translatedExcerpt *string
if translatedContent != nil {
    excerpt := extractPlainTextExcerpt(*translatedContent, 300)
    translatedExcerpt = &excerpt
}
```

Pass `TranslatedExcerpt: translatedExcerpt` to `SaveItemTranslation`.

**QA:** `go test -count=1 ./internal/handler/...` passes. Existing tests still green (add excerpt field to expected responses where needed).

---

- [x] Task 6 — Backend: Translation Settings Handler Update
**File:** `backend/internal/handler/translation_settings.go`

- Add `AutoTranslateMode bool` to `translationSettingsRequest` (json: `"auto_translate_mode"`)
- Add `AutoTranslateMode bool` to `translationSettingsResponse` (json: `"auto_translate_mode"`)
- In `patchTranslationSettings`: pass `AutoTranslateMode` to store update if present in request
- In `buildTranslationSettingsResponse`: include `AutoTranslateMode: settings.AutoTranslateMode`

**QA:** `go test -count=1 ./internal/handler/...` passes.

---

- [x] Task 7 — Backend: Batch Translate Endpoint
**File:** `backend/internal/handler/translation_item.go` (new handler `translateItemsBatch`)
**File:** `backend/internal/handler/handler.go` (register route)

### Endpoint
`POST /api/translation/items/batch`

Request body:
```json
{ "ids": [1, 2, 3] }
```

Response:
```json
{
  "data": {
    "translated": [1, 2],
    "failed": [3],
    "errors": { "3": "translated content failed validation" }
  }
}
```

### Implementation
```go
func (h *Handler) translateItemsBatch(c *gin.Context) {
    var req struct {
        IDs []int64 `json:"ids"`
    }
    if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
        badRequestError(c, "invalid request"); return
    }
    if len(req.IDs) > 20 {
        badRequestError(c, "max 20 items per batch"); return
    }
    // ... resolve settings, api key, model, lang
    // Run translations concurrently (max 5 goroutines) using errgroup or semaphore
    // Collect results: translated IDs, failed IDs + error messages
    // Return structured response
}
```

Concurrency: use a semaphore channel of size 5 to limit parallel goroutines.

Route: `router.POST("/api/translation/items/batch", authMiddleware, h.translateItemsBatch)`

**QA:** `go test -count=1 ./internal/handler/...` — add test `TestTranslateItemsBatch`:
- Seeds 3 items, stubs translator, calls batch endpoint, expects all 3 translated
- Test with `ids` > 20 → 400
- Test with empty `ids` → 400

---

- [x] Task 8 — Frontend: API Types Update
**File:** `frontend/src/lib/api/types.ts`

- Add `translated_excerpt?: string | null` to `Item` interface
- Add `auto_translate_mode?: boolean` to `TranslationSettings` interface
- Add `auto_translate_mode?: boolean` to `TranslationSettingsUpdateRequest`
- Add new types:
```typescript
export interface BatchTranslateRequest {
  ids: number[];
}
export interface BatchTranslateResult {
  translated: number[];
  failed: number[];
  errors: Record<string, string>;
}
```

**QA:** `npx tsc -b --noEmit` clean.

---

- [x] Task 9 — Frontend: API Client Update
**File:** `frontend/src/lib/api/index.ts`

Add to `translationAPI`:
```typescript
batchTranslate: async (ids: number[]) =>
  client.post<APIResponse<BatchTranslateResult>>("/api/translation/items/batch", { ids }),
```

**QA:** TypeScript compiles.

---

- [x] Task 10 — Frontend: Query Hooks Update
**File:** `frontend/src/queries/items.ts`

Add `useBatchTranslateItems` hook:
```typescript
export function useBatchTranslateItems() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (ids: number[]) => translationAPI.batchTranslate(ids),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.items.lists() });
    },
  });
}
```

**QA:** TypeScript compiles.

---

- [x] Task 11 — Frontend: Auto-Translate Batch Logic in Article List
**File:** `frontend/src/components/article/article-list.tsx`

When `autoTranslateMode` is ON (read from `useTranslationSettings()`):
1. After items load, find items where `translated_title == null && translated_content == null` (i.e., not yet translated)
2. Fire `useBatchTranslateItems` with their IDs in chunks of 10
3. Track which IDs are currently being translated (`translatingIds: Set<number>`) — pass this down to `ArticleItem`

Logic (inside the component, with `useEffect`):
```typescript
useEffect(() => {
  if (!autoTranslateMode || !isTranslationEnabled) return;
  const untranslated = articles.filter(a => !a.translated_title && !a.translated_content);
  if (untranslated.length === 0) return;
  // Chunk into groups of 10, fire sequentially
  const chunks = chunk(untranslated.map(a => a.id), 10);
  // setTranslatingIds(new Set(untranslated.map(a => a.id)));
  // fire batch for each chunk
}, [articles, autoTranslateMode]);
```

Pass `isTranslating: boolean` prop to `ArticleItem`.

**QA:** Visual — spinner appears next to date for untranslated items when mode is ON.

---

- [x] Task 12 — Frontend: Article Item — Show Translation + Spinner
**File:** `frontend/src/components/article/article-item.tsx`

Props changes:
```typescript
interface ArticleItemProps {
  // ... existing ...
  autoTranslateMode?: boolean;
  isTranslating?: boolean;
}
```

Logic:
```typescript
const displayTitle = autoTranslateMode && article.translated_title
  ? article.translated_title
  : article.title;

const displayExcerpt = autoTranslateMode && article.translated_excerpt
  ? article.translated_excerpt
  : extractSummary(article.content, 150);
```

Spinner (inline after date, between `·` separators):
```tsx
{isTranslating && (
  <>
    <span className="text-muted-foreground">·</span>
    <Loader2 className="h-3 w-3 animate-spin text-muted-foreground" />
  </>
)}
```

**QA:** Visual check — translated title and excerpt shown; spinner visible while translating.

---

- [x] Task 13 — Frontend: Article Drawer — Auto-Translate on Open
**File:** `frontend/src/components/article/article-drawer.tsx`

Current `useEffect` on line 171 resets `showTranslated=false`. Extend it:
```typescript
const autoTranslateMode = Boolean(translationSettings?.auto_translate_mode);

useEffect(() => {
  if (!selectedArticleId || !article) return;
  const hasTranslation = Boolean(article.translated_title || article.translated_content);
  if (autoTranslateMode && !hasTranslation && isTranslationEnabled && !translateItem.isPending) {
    void handleTranslate(false); // force=false — use cache if available
  }
  setShowTranslated(autoTranslateMode && hasTranslation);
}, [selectedArticleId, article?.id]);
```

Note: `setShowTranslated(true)` when auto mode ON AND translation exists → user sees translation immediately without extra click.

**QA:** Open an article that has a cached translation — it shows in translated mode automatically.
Open an article without translation — translation request fires, spinner visible in drawer header.

---

- [x] Task 14 — Frontend: Translation Settings UI — Auto-Translate Toggle
**File:** `frontend/src/components/settings/translation-settings-content.tsx`

Add Switch import from `@/components/ui/switch`.

Add state:
```typescript
const [autoTranslateMode, setAutoTranslateMode] = useState(false);
```

Initialize from settings:
```typescript
useEffect(() => {
  if (settings) {
    setAutoTranslateMode(settings.auto_translate_mode ?? false);
    // ... existing inits
  }
}, [settings]);
```

Include in `updatePayload`:
```typescript
if (autoTranslateMode !== (settings?.auto_translate_mode ?? false)) {
  updatePayload.auto_translate_mode = autoTranslateMode;
}
```

UI — add a section above the Update button:
```tsx
{/* Auto-Translate Mode Section */}
<div className="flex items-center justify-between">
  <div className="space-y-1">
    <p className="text-sm font-medium">{t("settings.translation.autoMode.label")}</p>
    <p className="text-[13px] text-muted-foreground">{t("settings.translation.autoMode.description")}</p>
  </div>
  <Switch
    checked={autoTranslateMode}
    onCheckedChange={setAutoTranslateMode}
    disabled={updateSettings.isPending || !settings?.has_api_key}
  />
</div>
```

**QA:** Toggle visible in settings, persists to DB after Save.

---

- [x] Task 15 — i18n: New Keys (All 8 Locales)
**Files:** `frontend/src/lib/i18n/messages/en.ts` (source of truth), then ru, zh, de, fr, es, pt, sv.

New keys to add:
```typescript
"settings.translation.autoMode.label": "Auto-translate mode",
"settings.translation.autoMode.description": "Automatically translate articles in the list and on open",
```

**QA:** `cd frontend && npm run check:i18n` — 0 errors.

---

- [x] Task 16 — Backend Tests: Batch Endpoint
**File:** `backend/internal/handler/translation_item_test.go`

Add `TestTranslateItemsBatch`:
- `seeds 3 items, stubs translator → all 3 translated, status 200`
- `ids > 20 → 400`
- `empty ids → 400`
- `one item fails (translator error) → partial success: translated=[1,2], failed=[3]`

**QA:** `go test -count=1 ./internal/handler/... -run TestTranslateItemsBatch` — all pass.

---

- [x] Task 17 — Final Build & Verification
1. `goimports -w .` in backend
2. `go build -o /dev/null ./...` — clean
3. `go test -count=1 ./...` — all green
4. `cd frontend && npx tsc -b --noEmit` — clean
5. `npm run check:i18n` — 0 errors
6. Full rebuild: `rm -rf frontend/dist backend/internal/web/dist build/fusion && ./scripts.sh build-frontend && ./scripts.sh build-backend`
7. Manual smoke test:
   - Enable auto-translate in settings → save
   - Navigate to HN Ask feed → untranslated items show original + spinner
   - Wait → items switch to translated title + excerpt
   - Open an article without translation → auto-translates
   - Open an already-translated article → shows translation immediately
   - Force re-translate → excerpt updates too

---

## Final Verification Wave
**STOP. Do not mark complete until user explicitly confirms the smoke test.**

Commit message:
```
feat: auto-translate mode — list titles/excerpts, drawer auto-translate, batch API
```
