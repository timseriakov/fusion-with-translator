# Translation Feature Work Plan

## TL;DR

> **Quick Summary**: Formalize and execute the existing `translation-feature.md` spec as a safe, test-driven translation feature for Fusion, preserving strict HTML guardrails and env-over-DB API key precedence.
>
> **Deliverables**:
> - Backend schema + store support for translation settings and cached item translations
> - Backend settings/models/translate API endpoints with validation and safe logging
> - Frontend settings UI and article translation UI wired to the new backend APIs
>
> **Estimated Effort**: Medium
> **Parallel Execution**: YES - 3 implementation waves + final verification wave
> **Critical Path**: Task 1 -> Task 2 -> Task 4/5/6 -> Task 7/8/9 -> Final Verification

---

## Context

### Original Request
Use `translation-feature.md` as the active implementation plan and begin work from it.

### Interview Summary
**Key Discussions**:
- Existing `translation-feature.md` is the accepted source specification.
- Scope boundaries from the spec are accepted as-is.
- User prefers TDD.
- Final verification wave should be included.

**Research Findings**:
- Backend already has strong Go test coverage using stdlib `testing`, subtests, fixture helpers, and `httptest`.
- Frontend currently has no automated test runner or UI test infrastructure.
- Repository CI already runs backend tests via `./scripts.sh test-backend`.

### Metis Review
**Identified Gaps** (addressed in this plan):
- Split the original broad tasks into smaller execution units to improve parallelism.
- Make backend/frontend verification strategy explicit instead of treating all work the same.
- Add explicit cache invalidation, failure-path, and HTML-preservation acceptance criteria.
- Add concrete evidence capture requirements for every QA scenario.
- Lock down logging, API masking, and mixed translation-state behavior as non-negotiable guardrails.

---

## Work Objectives

### Core Objective
Implement a user-triggered OpenAI-based article translation feature for Fusion that safely stores translation settings, caches per-model/per-language results, preserves article HTML structure, and exposes minimal UI controls in Settings and Article Drawer.

### Concrete Deliverables
- Translation schema migration for `ai_translation_settings` and `items` translation columns
- Store/model support for translation settings and persisted translation results
- `GET/PATCH /api/translation/settings`
- `GET /api/translation/models`
- `POST /api/translation/items/:id` with cache and `force=true` behavior
- Frontend translation API/query layer
- Minimal AI Translation settings section in Settings dialog
- Translate / Show translated / Show original / Re-translate article UI in drawer

### Definition of Done
- [ ] Backend tests for new store/handler logic pass via `cd backend && go test ./...`
- [ ] Frontend compiles via `cd frontend && npx tsc -b --noEmit`
- [ ] Frontend production build succeeds via `cd frontend && pnpm run build`
- [ ] Manual/API QA scenarios capture evidence under `.sisyphus/evidence/`
- [ ] Final verification wave approves plan compliance, code quality, QA coverage, and scope fidelity

### Must Have
- Strict API key precedence: ENV > DB > None
- Settings GET never returns raw API key
- Settings PATCH is partial, and empty string clears only the DB fallback key
- Translation cache is keyed by `translation_model` + `translation_target_language`
- `translation_updated_at` changes only after a successfully persisted new translation
- Valid HTML translation only: same tag order/count, unchanged attributes, ignored `script/style/code/pre`
- Title-only translation succeeds when content is empty/invalid
- Models endpoint degrades gracefully when key is missing or upstream is unavailable
- Article UI shows translated/original toggle when either translated field exists, and re-translate keeps previous translation visible until success
- No full article body or API key in logs

### Must NOT Have (Guardrails)
- No non-OpenAI providers
- No auto-translation during feed pull
- No optimistic UI for translation actions
- No raw API key exposure in API responses or logs
- No persistence when translated output fails validation or both translated fields are empty
- No direct modification of shadcn source files under `frontend/src/components/ui/`

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** - all verification is agent-executed.

### Test Decision
- **Infrastructure exists**: Backend YES, Frontend NO dedicated test runner
- **Automated tests**: Hybrid
- **Backend**: TDD
- **Frontend**: Tests-after not required; use agent-executed build + browser/API QA
- **Frameworks**: Go stdlib `testing`, `httptest`, SQLite temp DB helpers, frontend `tsc`/`pnpm build`/browser QA

### QA Policy
Every task includes executable QA scenarios with evidence capture.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Backend**: Go tests + curl verification
- **Frontend**: browser automation / UI checks + build verification
- **Database**: schema inspection and targeted query validation
- **Security-sensitive flows**: explicit masking/logging verification

---

## Execution Strategy

### Parallel Execution Waves

Wave 1 (Start Immediately - foundations):
├── Task 1: Translation schema migration [unspecified-high]
├── Task 2: Backend model + store translation primitives [unspecified-high]
└── Task 3: Frontend API/type/query scaffolding [quick]

Wave 2 (After Wave 1 - backend feature surface):
├── Task 4: Backend translation settings endpoints [unspecified-high]
├── Task 5: Backend OpenAI models endpoint [unspecified-high]
├── Task 6: Backend translation execution endpoint [unspecified-high]
└── Task 7: Frontend settings data wiring [quick]

Wave 3 (After Wave 2 - UI integration):
├── Task 8: Frontend settings UI [visual-engineering]
└── Task 9: Frontend article translation UI [visual-engineering]

Wave FINAL (After ALL implementation tasks — 4 parallel reviews, then user okay):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Real QA execution (unspecified-high)
└── Task F4: Scope fidelity check (deep)
-> Present results -> Get explicit user okay

Critical Path: 1 -> 2 -> 4/5/6 -> 7/8/9 -> F1-F4 -> user okay
Parallel Speedup: moderate but real; highest safe concurrency is constrained by API contract dependencies
Max Concurrent: 4

### Dependency Matrix
- **1**: None -> 2, 4, 5, 6
- **2**: 1 -> 4, 5, 6
- **3**: None (spec-driven scaffold) -> 7, 8, 9
- **4**: 1, 2 -> 7, 8
- **5**: 1, 2 -> 7, 8
- **6**: 1, 2 -> 7, 9
- **7**: 3, 4, 5, 6 -> 8, 9
- **8**: 4, 5, 7 -> FINAL
- **9**: 6, 7 -> FINAL
- **F1-F4**: 1-9 complete -> user approval

### Agent Dispatch Summary
- **Wave 1**: 3 agents
  - T1 -> `unspecified-high`
  - T2 -> `unspecified-high`
  - T3 -> `quick`
- **Wave 2**: 4 agents
  - T4 -> `unspecified-high`
  - T5 -> `unspecified-high`
  - T6 -> `unspecified-high`
  - T7 -> `quick`
- **Wave 3**: 2 agents
  - T8 -> `visual-engineering`
  - T9 -> `visual-engineering`
- **FINAL**: 4 agents
  - F1 -> `oracle`
  - F2 -> `unspecified-high`
  - F3 -> `unspecified-high`
  - F4 -> `deep`

---

## TODOs

---

- [x] 1. Translation schema migration

  **What to do**:
  - Add `backend/internal/store/migrations/004_ai_translation_settings.sql` to create `ai_translation_settings` and add translation columns to `items`.
  - Keep migration transactional and compatible with the existing embedded migration runner.
  - Add/extend migration tests so schema creation and repeated startup migration runs remain safe.

  **Must NOT do**:
  - Do not modify unrelated tables or search/bookmark schema.
  - Do not backfill translations or trigger translation work during migration.

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: schema work changes persistent state and must align with existing SQLite migration patterns.
  - **Skills**: [`git-master`]
    - `git-master`: useful for keeping migration/test changes atomic if commits are made later.
  - **Skills Evaluated but Omitted**:
    - `frontend-ui-ux`: no UI work in this task.

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 1
  - **Blocks**: Tasks 2, 4, 5, 6
  - **Blocked By**: None

  **References**:
  - `backend/internal/store/migrations/001_initial.sql` - canonical style for base table/index/trigger definitions.
  - `backend/internal/store/migrations/002_feed_fetch_state.sql` - current ALTER TABLE migration pattern and naming convention.
  - `backend/internal/store/migrate.go` - embedded migration discovery, ordering, and transactional application behavior.
  - `backend/internal/store/migrate_test.go` - existing migration verification patterns to extend for the new schema.

  **Acceptance Criteria**:
  - [ ] `004_ai_translation_settings.sql` exists and follows numeric migration naming.
  - [ ] `ai_translation_settings` table contains `id`, `openai_api_key`, `translation_model`, `translation_target_language`.
  - [ ] `items` contains `translated_title`, `translated_content`, `translation_model`, `translation_target_language`, `translation_updated_at`.
  - [ ] `cd backend && go test ./internal/store/...` passes with migration coverage updated.

  **QA Scenarios**:
  ```
  Scenario: migration creates translation schema on fresh DB
    Tool: Bash
    Preconditions: no existing temp database file
    Steps:
      1. Run `cd backend && go test ./internal/store/... -run 'TestMigrate|TestNew'`.
      2. Create a temp DB through the store test helper or app startup path.
      3. Run `sqlite3 <temp-db> ".schema ai_translation_settings"` and `sqlite3 <temp-db> ".schema items"`.
    Expected Result: translation table and item columns exist exactly once.
    Failure Indicators: missing table, missing columns, duplicate-column migration failure.
    Evidence: .sisyphus/evidence/task-1-schema.txt

  Scenario: migration remains idempotent on already-migrated DB
    Tool: Bash
    Preconditions: temp DB already migrated once
    Steps:
      1. Start migration path a second time against the same DB.
      2. Re-run the schema inspection commands.
    Expected Result: startup succeeds without duplicate-column or duplicate-table errors.
    Failure Indicators: migration crash, second schema version insertion, altered schema drift.
    Evidence: .sisyphus/evidence/task-1-idempotent.txt
  ```

  **Evidence to Capture**:
  - [ ] `.sisyphus/evidence/task-1-schema.txt`
  - [ ] `.sisyphus/evidence/task-1-idempotent.txt`

  **Commit**: NO

- [x] 2. Backend model and store translation primitives

  **What to do**:
  - Extend `model.Item` with translation fields and add a backend representation for translation settings state.
  - Add store methods to load/save translation settings, resolve persisted translation cache fields, and update an item's translated content metadata after successful translation.
  - Add or extend store tests first (TDD) using existing temp DB fixtures and subtest style.

  **Must NOT do**:
  - Do not couple store logic to OpenAI HTTP calls.
  - Do not store derived UI-only fields in backend models.

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: touches data contracts and persistence rules shared by handlers and frontend.
  - **Skills**: [`git-master`]
    - `git-master`: helps keep model/store/test updates logically grouped.
  - **Skills Evaluated but Omitted**:
    - `frontend-ui-ux`: no frontend behavior here.

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 1
  - **Blocks**: Tasks 4, 5, 6
  - **Blocked By**: Task 1

  **References**:
  - `backend/internal/model/model.go` - current JSON-facing model shape; add translation fields consistently here.
  - `backend/internal/store/item.go` - existing item query/scan/update patterns using named SQL parameters.
  - `backend/internal/store/store_test.go` - shared temp DB and fixture helpers for TDD-style store tests.
  - `backend/internal/store/item_test.go` - table-driven store test style to mirror for translation-specific cases.

  **Acceptance Criteria**:
  - [ ] New store methods exist for translation settings read/update and item translation persistence/cache retrieval.
  - [ ] `model.Item` exposes translation fields needed by API/frontend consumers.
  - [ ] Store tests cover cache-hit metadata, clearing DB API key with empty string, and successful translation persistence timestamp updates.
  - [ ] `cd backend && go test ./internal/store/...` passes.

  **QA Scenarios**:
  ```
  Scenario: store persists and reloads translation settings
    Tool: Bash
    Preconditions: backend store tests compiled against new schema
    Steps:
      1. Run `cd backend && go test ./internal/store/... -run 'Test.*Translation.*Settings' -v`.
      2. Confirm the test writes settings, reloads them, and verifies API key presence metadata without returning secrets in assertions.
    Expected Result: tests pass and persisted settings round-trip correctly.
    Failure Indicators: nil settings row handling broken, DB clear semantics broken, timestamp mismatch.
    Evidence: .sisyphus/evidence/task-2-settings-store.txt

  Scenario: store persists translation cache only after valid save
    Tool: Bash
    Preconditions: temp DB with one item row
    Steps:
      1. Run `cd backend && go test ./internal/store/... -run 'Test.*Translation.*Item' -v`.
      2. Verify test coverage includes cache metadata (`translation_model`, `translation_target_language`, `translation_updated_at`).
    Expected Result: tests prove valid translation saves update cache metadata and invalid/no-op cases do not.
    Failure Indicators: cache fields not returned, timestamp updated on invalid save, model/language mismatch ignored.
    Evidence: .sisyphus/evidence/task-2-item-store.txt
  ```

  **Evidence to Capture**:
  - [ ] `.sisyphus/evidence/task-2-settings-store.txt`
  - [ ] `.sisyphus/evidence/task-2-item-store.txt`

  **Commit**: NO

- [x] 3. Frontend translation API, type, and query scaffolding

  **What to do**:
  - Extend frontend API types for translation settings, model lists, and translated item fields.
  - Add API client helpers for translation settings/models/item translation endpoints.
  - Add query/mutation helpers used later by Settings and Article Drawer UI without styling work yet.

  **Must NOT do**:
  - Do not add a frontend test runner in this task.
  - Do not add UI markup or visual controls here.

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: isolated TypeScript contract wiring across a few files.
  - **Skills**: [`git-master`]
    - `git-master`: useful if the executor wants a clean atomic scaffold commit.
  - **Skills Evaluated but Omitted**:
    - `frontend-ui-ux`: styling and interaction design are deferred to later tasks.

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-2, but avoid depending on backend implementation details beyond the accepted spec)
  - **Blocks**: Tasks 7, 8, 9
  - **Blocked By**: None

  **References**:
  - `frontend/src/lib/api/types.ts` - existing API contract definitions matching backend JSON fields.
  - `frontend/src/lib/api/index.ts` - central API export surface for new translation endpoints.
  - `frontend/src/lib/api/client.ts` - shared request/error behavior for authenticated API calls.
  - `frontend/src/queries/items.ts` - current query/mutation patterns and optimistic-state boundaries to preserve.

  **Acceptance Criteria**:
  - [ ] Frontend types include translation fields on `Item` plus dedicated settings/models payloads.
  - [ ] API helpers exist for settings GET/PATCH, models GET, and item translation POST with optional `force=true`.
  - [ ] Query/mutation hooks compile and expose stable shapes for later UI tasks.
  - [ ] `cd frontend && npx tsc -b --noEmit` passes.

  **QA Scenarios**:
  ```
  Scenario: translation client/query scaffold compiles cleanly
    Tool: Bash
    Preconditions: frontend dependencies installed
    Steps:
      1. Run `cd frontend && npx tsc -b --noEmit`.
      2. Run `cd frontend && pnpm run build` to ensure query imports do not break production bundling.
    Expected Result: both commands succeed with no missing exports or type mismatches.
    Failure Indicators: TS errors for unknown translation fields, broken imports, or invalid mutation signatures.
    Evidence: .sisyphus/evidence/task-3-typescript.txt

  Scenario: translation API helper encodes optional force refresh
    Tool: Read + Bash
    Preconditions: translation helper code added under `frontend/src/lib/api/`
    Steps:
      1. Read `frontend/src/lib/api/index.ts` and confirm the translate-item helper targets `/translation/items/:id`.
      2. Confirm the helper appends `?force=true` only when the caller explicitly requests a forced refresh.
      3. Run `cd frontend && npx tsc -b --noEmit`.
    Expected Result: API helper supports both cache-first and forced-refresh calls without type errors.
    Failure Indicators: helper lacks a force option, always forces refresh, or builds an invalid route.
    Evidence: .sisyphus/evidence/task-3-api-shape.txt
  ```

  **Evidence to Capture**:
  - [ ] `.sisyphus/evidence/task-3-typescript.txt`
  - [ ] `.sisyphus/evidence/task-3-api-shape.txt`

  **Commit**: NO

- [x] 4. Backend translation settings endpoints

  **What to do**:
  - Add authenticated `GET /api/translation/settings` and `PATCH /api/translation/settings`.
  - Enforce `ENV > DB > None` runtime precedence and never return the raw key.
  - Support partial PATCH updates; empty string for DB key clears the fallback key.
  - Add handler tests first (TDD) covering env-backed and DB-backed cases.

  **Must NOT do**:
  - Do not expose `openai_api_key` in response bodies.
  - Do not make PATCH overwrite unspecified settings fields.

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: combines config/env semantics, request validation, and sensitive response shaping.
  - **Skills**: [`git-master`]
    - `git-master`: helps keep handler + store + test changes grouped if committed.
  - **Skills Evaluated but Omitted**:
    - `frontend-ui-ux`: no UI work.

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 5, 6, 7)
  - **Blocks**: Tasks 7, 8
  - **Blocked By**: Tasks 1, 2

  **References**:
  - `backend/internal/handler/handler.go` - router registration pattern for authenticated `/api` endpoints.
  - `backend/internal/handler/session.go` - request binding and JSON response style for auth-protected handlers.
  - `backend/internal/handler/errors.go` - shared error response/logging behavior to reuse.
  - `backend/internal/config/config.go` - current env loading approach; translation env key precedence must integrate consistently.
  - `.env.example` - where any new env key documentation should ultimately be reflected by the executor if implementation chooses env support.
  - `backend/internal/handler/test_helpers_test.go` - request helper utilities for TDD handler coverage.

  **Acceptance Criteria**:
  - [ ] `GET /api/translation/settings` returns `has_api_key`, `masked_api_key`, `api_key_source`, `translation_model`, `translation_target_language` without leaking secrets.
  - [ ] `PATCH /api/translation/settings` supports partial updates and clearing DB key with `""`.
  - [ ] ENV-backed key reports `api_key_source: "env"` even if a DB fallback exists.
  - [ ] `cd backend && go test ./internal/handler/...` passes with dedicated settings endpoint tests.

  **QA Scenarios**:
  ```
  Scenario: settings GET masks DB-backed key
    Tool: Bash
    Preconditions: backend server/test harness configured with DB-stored key and no env key
    Steps:
      1. Run `cd backend && go test ./internal/handler/... -run 'Test.*Translation.*Settings' -v`.
      2. Verify a GET response assertion checks `has_api_key=true`, `api_key_source="db"`, and a masked value instead of the raw secret.
    Expected Result: tests pass and no raw API key appears in response fixtures.
    Failure Indicators: raw key exposed, missing source field, missing partial update coverage.
    Evidence: .sisyphus/evidence/task-4-settings-handler.txt

  Scenario: env key overrides DB fallback at runtime
    Tool: Bash
    Preconditions: test harness provides both env key and DB key
    Steps:
      1. Run handler tests covering mixed env+db configuration.
      2. Confirm GET reports env as source and PATCH of key updates only fallback behavior, not runtime source.
    Expected Result: env precedence is preserved consistently.
    Failure Indicators: source incorrectly reports db, PATCH mutates runtime behavior, or GET hides source semantics.
    Evidence: .sisyphus/evidence/task-4-env-precedence.txt
  ```

  **Evidence to Capture**:
  - [ ] `.sisyphus/evidence/task-4-settings-handler.txt`
  - [ ] `.sisyphus/evidence/task-4-env-precedence.txt`

  **Commit**: NO

- [x] 5. Backend OpenAI models endpoint

  **What to do**:
  - Add authenticated `GET /api/translation/models` that uses the resolved API key source and fetches available OpenAI models.
  - Ensure graceful degradation: return an empty list or controlled 400-level error when key is missing or OpenAI is unavailable, without blocking the UI contract.
  - Add handler tests for missing key, upstream failure, and successful list mapping.

  **Must NOT do**:
  - Do not crash or return 500 for expected misconfiguration/upstream-unavailable cases.
  - Do not log secrets or full upstream payloads unnecessarily.

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: external API integration with graceful degradation and error classification.
  - **Skills**: [`git-master`]
    - `git-master`: helps isolate HTTP client / handler / tests as one unit if committed.
  - **Skills Evaluated but Omitted**:
    - `frontend-ui-ux`: this task is backend-only.

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 4, 6, 7)
  - **Blocks**: Tasks 7, 8
  - **Blocked By**: Tasks 1, 2

  **References**:
  - `backend/internal/handler/feed.go` - examples of external HTTP work plus graceful client-facing error handling.
  - `backend/internal/pkg/httpc/client.go` - existing internal HTTP client conventions and safety posture to align with where practical.
  - `backend/internal/handler/errors.go` - reuse controlled error responses rather than leaking upstream details.
  - `backend/internal/handler/test_helpers_test.go` - request execution helpers for handler tests.

  **Acceptance Criteria**:
  - [ ] Missing API key does not produce an uncontrolled 500; endpoint degrades per spec.
  - [ ] Successful upstream call returns a frontend-consumable model list.
  - [ ] Upstream transport/error cases are converted to safe, predictable responses.
  - [ ] `cd backend && go test ./internal/handler/...` passes with models endpoint coverage.

  **QA Scenarios**:
  ```
  Scenario: models endpoint degrades gracefully without configured key
    Tool: Bash
    Preconditions: no env key and no DB key
    Steps:
      1. Run `cd backend && go test ./internal/handler/... -run 'Test.*Translation.*Models' -v`.
      2. Verify assertions cover either empty-list success or controlled 400 behavior, per chosen implementation.
    Expected Result: handler tests pass and the endpoint contract remains usable by UI fallback flows.
    Failure Indicators: unexpected 500, panic, or secret leakage in error body.
    Evidence: .sisyphus/evidence/task-5-models-no-key.txt

  Scenario: models endpoint maps upstream success payload correctly
    Tool: Bash
    Preconditions: handler test with mocked/fake OpenAI response
    Steps:
      1. Run the same focused models handler tests.
      2. Confirm returned model identifiers are extracted into the expected API shape.
    Expected Result: tests prove valid model list mapping and stable response semantics.
    Failure Indicators: malformed list, wrong field names, or transport errors treated as success.
    Evidence: .sisyphus/evidence/task-5-models-success.txt
  ```

  **Evidence to Capture**:
  - [ ] `.sisyphus/evidence/task-5-models-no-key.txt`
  - [ ] `.sisyphus/evidence/task-5-models-success.txt`

  **Commit**: NO

- [x] 6. Backend translation execution endpoint

  **What to do**:
  - Add authenticated `POST /api/translation/items/:id` with optional `force=true`.
  - Resolve translation settings, return cached translation when model/language match and `force` is absent, and perform a new translation otherwise.
  - Translate title as plain text; translate only HTML text nodes for valid content while ignoring `script`, `style`, `code`, and `pre`.
  - Validate translated HTML by reparsing and ensuring tag count/order and attributes are preserved before persisting.
  - Add handler/service tests first (TDD) for cache hits, force bypass, invalid HTML, title-only success, and failure when both translated outputs are empty.

  **Must NOT do**:
  - Do not persist failed validation output.
  - Do not log article bodies or API keys.
  - Do not mutate original article content in storage.

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: most complex business logic in the feature; combines cache semantics, DOM rules, and external API handling.
  - **Skills**: [`git-master`]
    - `git-master`: useful for keeping large handler/service/test changes reviewable if committed.
  - **Skills Evaluated but Omitted**:
    - `frontend-ui-ux`: not relevant.

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 4, 5, 7)
  - **Blocks**: Tasks 7, 9
  - **Blocked By**: Tasks 1, 2

  **References**:
  - `backend/internal/handler/item.go` - existing item endpoint structure and auth-protected resource parsing pattern.
  - `backend/internal/store/item.go` - item fetch/update flow and SQL scanning conventions for translation cache reads/writes.
  - `backend/internal/handler/errors.go` - safe error/logging surface.
  - `backend/internal/handler/item_test.go` - handler test style using subtests and request helpers.
  - `translation-feature.md` - accepted source spec for cache semantics, DOM translation guardrails, and mixed title/content outcomes.

  **Acceptance Criteria**:
  - [ ] Matching cached translation is returned when `force` is absent and model/language match current settings.
  - [ ] `force=true` bypasses cache and only updates `translation_updated_at` after a valid new translation is persisted.
  - [ ] Invalid/empty HTML content can still yield a valid title-only translation with `translated_content=NULL`.
  - [ ] If translated HTML fails parse/structure validation, nothing is saved and an error is returned.
  - [ ] If both translated title and translated content are empty, endpoint returns an error and does not persist.
  - [ ] `cd backend && go test ./internal/handler/... ./internal/store/...` passes with translation endpoint coverage.

  **QA Scenarios**:
  ```
  Scenario: cached translation is returned for matching model/language
    Tool: Bash
    Preconditions: item already has translation fields matching current settings
    Steps:
      1. Run `cd backend && go test ./internal/handler/... -run 'Test.*Translation.*Cache' -v`.
      2. Verify the test asserts no fresh upstream translation call occurs for the cache-hit path.
    Expected Result: endpoint returns cached translated fields unchanged.
    Failure Indicators: unnecessary upstream call, timestamp churn, or cache ignored.
    Evidence: .sisyphus/evidence/task-6-cache-hit.txt

  Scenario: force refresh bypasses cache and preserves validation rules
    Tool: Bash
    Preconditions: item already has cached translation and handler test can trigger `?force=true`
    Steps:
      1. Run focused handler tests covering `force=true`.
      2. Confirm the refreshed translation replaces cached fields only after validation passes.
    Expected Result: cache bypass occurs and persisted output changes only on valid translation.
    Failure Indicators: stale cache returned despite force, invalid output persisted, timestamp updated on failure.
    Evidence: .sisyphus/evidence/task-6-force-refresh.txt

  Scenario: invalid HTML content falls back to title-only translation
    Tool: Bash
    Preconditions: item content is empty or malformed HTML fragment
    Steps:
      1. Run focused translation handler tests for malformed/empty content.
      2. Confirm response and persistence keep `translated_content` null while saving translated title.
    Expected Result: request succeeds if title translation exists.
    Failure Indicators: hard failure on malformed content, content field populated with invalid HTML.
    Evidence: .sisyphus/evidence/task-6-title-only.txt

  Scenario: structurally-invalid translated HTML is rejected
    Tool: Bash
    Preconditions: mocked upstream translation changes tag order/count or attributes
    Steps:
      1. Run focused translation validation tests.
      2. Confirm the endpoint returns an error and no cache fields are updated.
    Expected Result: persistence is blocked on structural validation failure.
    Failure Indicators: invalid HTML saved, original attributes altered, silent success.
    Evidence: .sisyphus/evidence/task-6-invalid-html.txt
  ```

  **Evidence to Capture**:
  - [ ] `.sisyphus/evidence/task-6-cache-hit.txt`
  - [ ] `.sisyphus/evidence/task-6-force-refresh.txt`
  - [ ] `.sisyphus/evidence/task-6-title-only.txt`
  - [ ] `.sisyphus/evidence/task-6-invalid-html.txt`

  **Commit**: NO

- [x] 7. Frontend translation data wiring

  **What to do**:
  - Connect frontend query/mutation helpers to the finalized backend contracts for settings/models/translate flows.
  - Add cache invalidation and local query refresh behavior needed by upcoming UI tasks.
  - Keep state boundaries clear: translation action state is local at the drawer level, while persisted settings live in query cache.

  **Must NOT do**:
  - Do not introduce optimistic translation UI.
  - Do not add finished visual presentation here beyond minimal plumbing if unavoidable.

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: contract wiring and cache invalidation across a small set of TS files.
  - **Skills**: [`git-master`]
    - `git-master`: helpful for atomic wiring changes if committed.
  - **Skills Evaluated but Omitted**:
    - `frontend-ui-ux`: the main UI layout comes in Tasks 8 and 9.

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 4, 5, 6)
  - **Blocks**: Tasks 8, 9
  - **Blocked By**: Tasks 3, 4, 5, 6

  **References**:
  - `frontend/src/queries/items.ts` - current query invalidation and detail/list synchronization patterns.
  - `frontend/src/lib/api/index.ts` - endpoint helper composition.
  - `frontend/src/lib/api/client.ts` - shared API error behavior that UI should consume.
  - `frontend/src/queries/keys.ts` - query key organization to extend for translation settings/models.

  **Acceptance Criteria**:
  - [ ] Settings queries/mutations and translate-item mutation are wired to final backend response shapes.
  - [ ] Relevant item/settings queries invalidate or refresh after mutation success.
  - [ ] TypeScript build passes with no unused/incorrect translation imports.
  - [ ] `cd frontend && npx tsc -b --noEmit` passes.

  **QA Scenarios**:
  ```
  Scenario: query wiring compiles and invalidation hooks are connected
    Tool: Bash
    Preconditions: backend contract helpers already present
    Steps:
      1. Run `cd frontend && npx tsc -b --noEmit`.
      2. Run `cd frontend && pnpm run build`.
    Expected Result: compile/build succeed after query key and mutation wiring changes.
    Failure Indicators: stale imports, missing invalidation dependencies, broken hook return types.
    Evidence: .sisyphus/evidence/task-7-query-wiring.txt

  Scenario: translation mutation path refreshes item consumers after success
    Tool: Read + Bash
    Preconditions: translation settings/model/item hooks implemented
    Steps:
      1. Read `frontend/src/queries/items.ts` and `frontend/src/queries/keys.ts` to confirm translation success invalidates or refreshes the affected item detail/list queries.
      2. Confirm the mutation path also refreshes any translation settings/models queries it updates.
      3. Run `cd frontend && npx tsc -b --noEmit`.
    Expected Result: drawer consumers have a defined refetch/invalidation path after successful translation.
    Failure Indicators: no invalidation, stale item cache path, or disconnected query keys.
    Evidence: .sisyphus/evidence/task-7-cache-refresh.txt
  ```

  **Evidence to Capture**:
  - [ ] `.sisyphus/evidence/task-7-query-wiring.txt`
  - [ ] `.sisyphus/evidence/task-7-cache-refresh.txt`

  **Commit**: NO

- [x] 8. Frontend settings UI

  **What to do**:
  - Add a minimal “AI Translation” section inside `SettingsDialog`.
  - Show key presence state, masked key text, and `configured by environment` behavior when source is env.
  - Add target language input/default and model combo-box UX with optional fetch-models action and manual fallback.
  - Surface loading/error states without blocking the rest of Settings.

  **Must NOT do**:
  - Do not expose the raw API key value.
  - Do not redesign the full Settings dialog or modify shadcn internals.
  - Do not block settings usage if model fetching fails.

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: requires fitting a new settings subsection cleanly into an existing dialog UX.
  - **Skills**: [`frontend-ui-ux`, `git-master`]
    - `frontend-ui-ux`: useful for adding compact, polished controls into the existing settings layout.
    - `git-master`: keeps UI/i18n/api-wiring changes atomic if committed.
  - **Skills Evaluated but Omitted**:
    - `dev-browser`: useful during execution QA, but not needed as a planning skill entry.

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Task 9)
  - **Blocks**: Final verification
  - **Blocked By**: Tasks 4, 5, 7

  **References**:
  - `frontend/src/components/settings/settings-dialog.tsx` - existing dialog structure, nav tabs, and section layout to extend minimally.
  - `frontend/src/lib/i18n.ts` and `frontend/src/lib/i18n/messages/*.ts` - translation key patterns for new settings copy.
  - `frontend/src/lib/api/client.ts` - error handling shape consumed by the UI.

  **Acceptance Criteria**:
  - [ ] Settings dialog contains an AI Translation section with model + target language controls.
  - [ ] Env-backed key state is clearly marked as environment-configured/read-only or equivalent fallback UX.
  - [ ] Model fetch failure leaves manual input usable.
  - [ ] `cd frontend && npx tsc -b --noEmit` and `cd frontend && pnpm run build` both pass.

  **QA Scenarios**:
  ```
  Scenario: AI Translation section renders in Settings dialog
    Tool: Playwright
    Preconditions: app is running locally and an authenticated session can open the Settings dialog
    Steps:
      1. Open the Settings dialog and locate the dialog container with `getByRole("dialog")`.
      2. Inside the dialog, assert there is a control labeled `Translation model` and a control labeled `Target language`.
      3. Assert the section also exposes API key status text (for example `API key configured`, `No API key`, or equivalent explicit status copy).
    Expected Result: the new section appears inside Settings without breaking the existing dialog layout.
    Failure Indicators: dialog fails to open, controls are missing, or layout overflows/breaks existing settings sections.
    Evidence: .sisyphus/evidence/task-8-settings-ui.png

  Scenario: env-configured key is shown without revealing the secret
    Tool: Playwright
    Preconditions: backend returns `api_key_source="env"` with masked key metadata
    Steps:
      1. Open the Settings dialog and navigate to the AI Translation section.
      2. Assert visible text communicates `Configured by environment` (or the finalized equivalent copy).
      3. Assert the key field is read-only/disabled and does not display the full raw secret value.
    Expected Result: UI clearly communicates env ownership while keeping the secret hidden.
    Failure Indicators: editable env-backed key field, raw key visible, or no source hint.
    Evidence: .sisyphus/evidence/task-8-env-key-ui.png

  Scenario: model fetch failure still allows manual entry
    Tool: Playwright
    Preconditions: models endpoint returns controlled failure or an empty list
    Steps:
      1. Open the AI Translation section and click the `Fetch models` button using `getByRole("button", { name: "Fetch models" })`.
      2. Assert a non-blocking error/empty-state message appears.
      3. Fill the `Translation model` field manually and save/update settings.
    Expected Result: manual model entry remains usable despite fetch failure.
    Failure Indicators: manual input becomes disabled, dialog crashes, or save path is blocked by failed fetch.
    Evidence: .sisyphus/evidence/task-8-model-fallback.png
  ```

  **Evidence to Capture**:
  - [ ] `.sisyphus/evidence/task-8-settings-ui.png`
  - [ ] `.sisyphus/evidence/task-8-env-key-ui.png`
  - [ ] `.sisyphus/evidence/task-8-model-fallback.png`

  **Commit**: NO

- [x] 9. Frontend article translation UI

  **What to do**:
  - Add translation controls to `ArticleDrawer` for Translate, Show translated / Show original, and Re-translate.
  - Keep translation action state local to the drawer/article view.
  - Show loading spinner/disabled state during translation, inline error feedback on failure, and preserve previous translation as fallback until re-translate succeeds.
  - Render translated title/content whenever available, including mixed state where only one translated field exists.

  **Must NOT do**:
  - Do not add optimistic translation rendering before the backend succeeds.
  - Do not remove access to original article content.
  - Do not break existing read/star/navigation drawer controls.

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: interactive drawer-state work with compact action design and mixed-content rendering.
  - **Skills**: [`frontend-ui-ux`, `git-master`]
    - `frontend-ui-ux`: helps fit new controls into the drawer header/content without clutter.
    - `git-master`: keeps UI/i18n/query-state changes grouped if committed.
  - **Skills Evaluated but Omitted**:
    - `dev-browser`: execution-phase QA tool, not necessary in the plan profile.

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Task 8)
  - **Blocks**: Final verification
  - **Blocked By**: Tasks 6, 7

  **References**:
  - `frontend/src/components/article/article-drawer.tsx` - existing action button area, content rendering, and footer navigation layout to preserve.
  - `frontend/src/queries/items.ts` - item detail and mutation interaction patterns.
  - `frontend/src/lib/content.ts` - existing article content processing path that translated content must remain compatible with.
  - `translation-feature.md` - source guardrails for mixed translation state, local state, and retry behavior.

  **Acceptance Criteria**:
  - [ ] Drawer shows Translate when no translation exists.
  - [ ] Drawer shows Show translated / Show original toggle whenever either translated title or translated content exists.
  - [ ] Re-translate keeps prior translation visible until a new successful translation replaces it.
  - [ ] Error state is visible inline/non-blocking and loading state disables repeat actions.
  - [ ] `cd frontend && npx tsc -b --noEmit` and `cd frontend && pnpm run build` pass.

  **QA Scenarios**:
  ```
  Scenario: first-time translation flow updates drawer state
    Tool: Playwright
    Preconditions: an article drawer is already open for an item with no existing translation and valid translation settings
    Steps:
      1. In the drawer, click `getByRole("button", { name: "Translate" })`.
      2. Assert the same button becomes disabled and shows a loading state.
      3. Wait for success and assert either translated title text or translated article content is visible.
      4. Assert a `Show original` or `Show translated` toggle is now present.
    Expected Result: translation appears and original content remains accessible via the toggle.
    Failure Indicators: no loading state, drawer closes unexpectedly, translated content never appears, or no toggle appears after success.
    Evidence: .sisyphus/evidence/task-9-translate-success.png

  Scenario: translation failure shows inline feedback without corrupting the drawer
    Tool: Playwright
    Preconditions: backend translation endpoint returns a controlled error for the opened article
    Steps:
      1. Click `getByRole("button", { name: "Translate" })`.
      2. Assert an inline error message or toast appears with translation failure feedback.
      3. Assert the original article content is still visible and the Translate action becomes usable again after the failure.
    Expected Result: failure is visible but non-destructive.
    Failure Indicators: blank drawer, stuck loading state, or silent failure with no user feedback.
    Evidence: .sisyphus/evidence/task-9-translate-error.png

  Scenario: re-translate preserves previous translation until replacement succeeds
    Tool: Playwright
    Preconditions: an article drawer is open for an item that already has translated title or content
    Steps:
      1. Confirm translated content is currently visible.
      2. Click `getByRole("button", { name: "Re-translate" })`.
      3. While the request is in flight, assert the previous translated content remains visible as the fallback state.
      4. After success, assert the updated translated content replaces the old translated content and the original toggle still works.
    Expected Result: no flicker back to untranslated-only state during retry, and prior translation survives until success.
    Failure Indicators: translated content disappears during retry, old translation is lost on failed retry, or the toggle stops working.
    Evidence: .sisyphus/evidence/task-9-retranslate-fallback.png
  ```

  **Evidence to Capture**:
  - [ ] `.sisyphus/evidence/task-9-translate-success.png`
  - [ ] `.sisyphus/evidence/task-9-translate-error.png`
  - [ ] `.sisyphus/evidence/task-9-retranslate-fallback.png`

  **Commit**: NO

---

## Final Verification Wave

> 4 review agents run in parallel. All must approve before work is considered ready for user sign-off.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Verify all Must Have / Must NOT Have items against final implementation and evidence artifacts.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run backend tests, frontend typecheck/build, and inspect changed files for error handling, naming quality, logging safety, and AI slop.
  Output: `Backend [PASS/FAIL] | Frontend [PASS/FAIL] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Real QA Execution** — `unspecified-high`
  Execute every documented QA scenario, capture evidence, and validate cross-task integration.
  Output: `Scenarios [N/N] | Integration [PASS/FAIL] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  Compare final diff against task specs, guardrails, and exclusions. Flag missing work or unauthorized scope growth.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | VERDICT`

---

## Commit Strategy

- **Wave 1**: `feat(translation): add schema and persistence foundation`
- **Wave 2**: `feat(translation): add backend translation APIs`
- **Wave 3**: `feat(translation): add translation UI flows`
- Prefer one commit per wave unless implementation naturally lands as smaller atomic commits.

---

## Success Criteria

### Verification Commands
```bash
cd backend && go test ./...                     # Expected: PASS
cd frontend && npx tsc -b --noEmit             # Expected: no errors
cd frontend && pnpm run build                  # Expected: successful production build
sqlite3 fusion.db ".schema ai_translation_settings"  # Expected: table exists after migration
```

### Final Checklist
- [ ] All required translation schema exists
- [ ] Settings endpoint masks API keys and reports source correctly
- [ ] Translation endpoint respects cache + force semantics
- [ ] HTML translation preserves DOM structure and attributes
- [ ] Frontend exposes settings and article translation controls correctly
- [ ] Final verification wave approves all work
