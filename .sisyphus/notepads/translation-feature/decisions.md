# Design Decisions

## 1. Translation Mutation Cache Invalidation Scope
**Decision**: Invalidate both item queries AND translation queries on translate success.

**Rationale**:
- Item queries ensure UI shows updated translations (drawer, feed lists)
- Translation queries ensure settings/models sync with user configuration
- Separate invalidation keeps scope tight but comprehensive

**Impact**: Critical for consistent UX - prevents stale data in multiple contexts.

## 2. Separate Mutation for Settings Update
**Decision**: Use dedicated `useUpdateTranslationSettings` mutation instead of reusing translate mutation.

**Rationale**:
- Settings updates don't affect item translations, so no item query invalidation needed
- Clear separation of concerns
- Settings changes can be validated before invalidating any queries

**Impact**: More targeted cache updates, less unnecessary refetching.

## 3. Local State for Translation Actions
**Decision**: Keep translation action state (loading/error) local to drawer/component, not global store.

**Rationale**:
- Each drawer has independent translation state
- Prevents UI conflicts across multiple open drawers
- Aligns with "minimal state boundary" principle from spec

**Impact**: Simpler state management, no race conditions between multiple translations.

## Task 8 Design Decisions

**Component Structure:**
- Created separate `TranslationSettingsContent` component instead of inline content
- Chose inline "Wrench" icon from lucide-react instead of searching for alternative
- Used modular imports for UI components (Button from ui/button, Input/Select from ui/input/select) for clarity

**UX Patterns:**
- API key status shown with color-coded icons (green for configured, yellow for unconfigured)
- Environment source explicitly communicated with sourceEnv/sourceDb labels
- Model selector fallback to manual input with fetch button for updates
- Target language control with predefined language list (RU, EN, ZH, DE, FR, ES, PT, SV, JA, KO)
- Placed translation controls in the header of `ArticleDrawer` for easy access.
- Used a secondary `RefreshCw` button for re-translation to keep the UI clean.
- Decided to show the "Translate" button only if an AI API key is configured.
