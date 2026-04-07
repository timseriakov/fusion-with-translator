# Work Plan: Transition RSS Reader to Markdown

This plan outlines the transition from storing and rendering raw HTML to Markdown. This will improve security, simplify translation logic, and provide a more consistent reading experience.

## TODOs

- [x] 1.1 Add Dependencies
    - [x] Add `github.com/JohannesKaufmann/html-to-markdown/v2` to `backend/go.mod`.
    - [x] Run `go mod tidy`.
- [ ] 1.2 Markdown Utility
    - [ ] Create `backend/internal/pkg/md/md.go`.
    - [ ] Implement `FromHTML(html string) (string, error)` with iframe preservation.
    - [ ] Add unit tests in `backend/internal/pkg/md/md_test.go`.
- [ ] 1.3 Update Feed Parser
    - [ ] Modify `backend/internal/pull/parser.go` to convert content to Markdown in `mapItem`.
    - [ ] Ensure `fallbackGUID` remains stable.
- [ ] 1.4 Simplify Translation Logic
    - [ ] Modify `backend/internal/handler/translation_item.go` to translate Markdown directly.
    - [ ] Remove complex HTML placeholder logic.
- [ ] 2.1 Data Migration
    - [ ] Create migration `backend/internal/store/migrations/006_convert_html_to_markdown.sql` (or Go-based migration).
    - [ ] Convert existing `content` and `translated_content` to Markdown.
- [ ] 3.1 Frontend Dependencies
    - [ ] Add `react-markdown`, `remark-gfm`, `rehype-raw` to `frontend/package.json`.
- [ ] 3.2 Markdown Renderer Component
    - [ ] Create `frontend/src/components/article/markdown-renderer.tsx`.
- [ ] 3.3 Update ArticleDrawer
    - [ ] Switch to `<MarkdownRenderer />` in `frontend/src/components/article/article-drawer.tsx`.
- [ ] 3.4 Update Summary Extraction
    - [ ] Update `extractSummary` in `frontend/src/lib/utils.ts` for Markdown.

## Final Verification Wave

- [ ] F1. Plan compliance audit (oracle)
- [ ] F2. Code quality review (unspecified-high)
- [ ] F3. Real QA execution (unspecified-high)
- [ ] F4. Scope fidelity check (deep)
