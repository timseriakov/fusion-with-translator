# Learnings from Translation Feature

## Frontend Query Invalidation Patterns

**What**: Translation mutations successfully invalidate query caches when items are translated or settings are updated.

**Where**: `frontend/src/queries/items.ts` (lines 250-296)

**Learned**:
- `useTranslateItem` mutation must invalidate BOTH item queries AND translation queries
  - Item detail query: `queryKeys.items.detail(itemId)` - ensures drawer shows updated translation
  - Item list queries: `queryKeys.items.lists()` - ensures feed lists show updated translations
  - Translation settings: `queryKeys.translation.settings()` - ensures settings UI shows new values
  - Translation models: `queryKeys.translation.models()` - ensures models refresh after changes

- `useUpdateTranslationSettings` mutation should only invalidate settings query
  - Settings changes don't affect items, only the settings configuration
  - Keep scope tight to avoid unnecessary refetches

- Query key structure follows established patterns
  - Translation keys are nested under `queryKeys.translation`
  - `settings()` and `models()` return array literals for TanStack Query
  - Consistent with existing `items.detail()` and `items.lists()` patterns

## TypeScript Type Safety

**What**: Translation hooks are fully type-safe with no runtime errors.

**Where**: All frontend translation files

**Learned**:
- TypeScript compilation passes without errors after wiring
- All mutation return types match API response shapes from backend
- Query keys are typed as const arrays, preventing typos
- API helpers use typed responses from `translationAPI.*` functions

## Cache Invalidation Strategy

**What**: Proper cache invalidation prevents stale translation data in UI.

**Where**: Translation mutation `onSuccess` callbacks

**Learned**:
- Item detail queries MUST invalidate when translation succeeds
  - Prevents showing stale "Show original" when translation is now available
  - Ensures drawer updates with new translated content

- Item list queries MUST invalidate when translation succeeds
  - Prevents showing old translated content in feed lists
  - Critical for batch translation scenarios

- Translation settings queries MUST invalidate on update
  - Ensures settings panel reflects new values immediately
  - Required for model/language changes to take effect

## Minimal State Boundary

**What**: Translation action state remains local to drawer/component, not global.

**Where**: Planning spec (Task 7)

**Learned**:
- `useTranslateItem` and `useUpdateTranslationSettings` are standalone hooks
- No global store state for translation actions
- Drawer component manages its own loading/error states
- Local state prevents UI flickering during translation

## Task 8 Completion Learnings

**Files Modified:**
- `frontend/src/components/settings/settings-dialog.tsx` - Added AI Translation tab with Wrench icon
- `frontend/src/components/settings/translation-settings-content.tsx` - New component for translation settings UI
- `frontend/src/lib/i18n/messages/*.ts` - Added 25 translation keys across all 8 locales

**Key Learnings:**
1. **Component Import Paths**: UI components must be imported from explicit paths (`@/components/ui/button` not `@/components/ui`) to avoid type resolution errors
2. **lucide-react Icon Availability**: Not all icons are guaranteed to exist in the pre-built package. Used Wrench instead of Translate for settings icon
3. **i18n Key Management**: When adding multiple keys across locales, use Python scripts for bulk insertion to avoid syntax errors from sed's complex escape handling
4. **TypeScript Implicit Any**: React event handlers need explicit type annotations (e.g., `React.ChangeEvent<HTMLInputElement>`, `(value: string) =>`)
5. **Duplicate Key Prevention**: Python's set-based deduplication is safer than sed for removing duplicate object-literal keys

## Task 8 Final Fixes

**State Management:**
- Separated `translationModel` and `translationTargetLanguage` state fields
- Used `useEffect` to initialize UI controls from fetched settings on mount
- Only sends `translation_model` to update payload if changed from current settings
- Detects no-op saves with "No changes to save" toast

**Error Handling:**
- Removed debug console.error calls, using only user-facing toasts
- Changed unused error parameter to `_error` to suppress TypeScript warnings
- Model fetch errors don't block manual model entry (shown below controls)

**Translation Model Controls:**
- Manual input updates `translationModel` state directly
- Select from fetched list updates `translationModel` state via `onValueChange`
- Model fetch failure renders red error message but doesn't disable manual input
- Integrated translation UI into `ArticleDrawer` using `useTranslateItem` and `useTranslationSettings` hooks.
- Managed local state for `showTranslated` to toggle between original and translated content.
- Used `useEffect` to reset translation view state when switching articles.
