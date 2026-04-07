package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/0x2E/fusion/internal/model"

	"github.com/0x2E/fusion/internal/config"
	"github.com/0x2E/fusion/internal/pkg/httpc"
	"github.com/0x2E/fusion/internal/store"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const openAIChatURL = "https://api.openai.com/v1/chat/completions"

var errTranslationUnavailable = errors.New("translation unavailable")

type itemTranslator interface {
	Translate(ctx context.Context, apiKey, model, systemPrompt, userPrompt string) (string, error)
}

type openAIItemTranslator struct {
	client *http.Client
}

func newItemTranslator(cfg *config.Config) itemTranslator {
	allowPrivateFeeds := cfg != nil && cfg.AllowPrivateFeeds
	client, err := httpc.NewClient(120*time.Second, "", allowPrivateFeeds)
	if err != nil {
		return &openAIItemTranslator{client: http.DefaultClient}
	}
	return &openAIItemTranslator{client: client}
}

type openAIChatRequest struct {
	Model     string              `json:"model"`
	Messages  []openAIChatMessage `json:"messages"`
	MaxTokens int                 `json:"max_tokens,omitempty"`
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatResponse struct {
	Choices []openAIChatChoice `json:"choices"`
}

type openAIChatChoice struct {
	Message openAIChatMessage `json:"message"`
}

func (t *openAIItemTranslator) Translate(ctx context.Context, apiKey, model, systemPrompt, userPrompt string) (string, error) {
	if t == nil || t.client == nil {
		return "", fmt.Errorf("translate: %w", errTranslationUnavailable)
	}

	payload := openAIChatRequest{
		Model: model,
		Messages: []openAIChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens: 4096,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal chat request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIChatURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build chat request: %w", err)
	}

	httpc.SetDefaultHeaders(req)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("chat request: %w", errTranslationUnavailable)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("chat request status %d: %w", resp.StatusCode, errTranslationUnavailable)
	}

	var decoded openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", fmt.Errorf("decode chat response: %w", errTranslationUnavailable)
	}
	if len(decoded.Choices) == 0 {
		return "", fmt.Errorf("empty chat choices: %w", errTranslationUnavailable)
	}

	return strings.TrimSpace(decoded.Choices[0].Message.Content), nil
}

type textNodeInfo struct {
	Placeholder string
	Leading     string
	Core        string
	Trailing    string
}

type htmlNodeInfo struct {
	Tag        string
	Attributes string
}

func (h *Handler) translateItem(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		badRequestError(c, "invalid id")
		return
	}

	force := false
	if rawForce := strings.TrimSpace(c.Query("force")); rawForce != "" {
		force, err = strconv.ParseBool(rawForce)
		if err != nil {
			badRequestError(c, "invalid force")
			return
		}
	}

	settings, err := h.store.GetTranslationSettings()
	if err != nil {
		internalError(c, err, "get translation settings for item")
		return
	}

	apiKey, _ := h.resolveTranslationAPIKey(settings.OpenAIAPIKey)
	if apiKey == "" {
		badRequestError(c, "no API key configured")
		return
	}
	if strings.TrimSpace(settings.TranslationModel) == "" {
		badRequestError(c, "no translation model configured")
		return
	}
	if strings.TrimSpace(settings.TranslationTargetLanguage) == "" {
		badRequestError(c, "no target language configured")
		return
	}

	if err := h.doTranslateItem(c.Request.Context(), id, settings, apiKey, force); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			notFoundError(c, "item")
			return
		}
		if errors.Is(err, errTranslationProducedNoOutput) || errors.Is(err, errTranslatedHTMLInvalid) {
			badRequestError(c, err.Error())
			return
		}
		internalError(c, err, "translate item")
		return
	}

	updatedItem, err := h.store.GetItem(id)
	if err != nil {
		internalError(c, err, "get translated item")
		return
	}

	dataResponse(c, updatedItem)
}

var errTranslationProducedNoOutput = errors.New("translation produced no output")

func (h *Handler) doTranslateItem(ctx context.Context, id int64, settings *model.TranslationSettings, apiKey string, force bool) error {
	item, err := h.store.GetItem(id)
	if err != nil {
		return err
	}

	if !force {
		cache, err := h.store.GetItemTranslationCache(id)
		if err == nil && cache.TranslationModel == settings.TranslationModel && cache.TranslationTargetLanguage == settings.TranslationTargetLanguage && cache.TranslationUpdatedAt > 0 && (cache.TranslatedTitle != nil || cache.TranslatedContent != nil) {
			// Cache miss if content exists but was never translated (e.g. cached before plain-text fallback was added)
			contentNeedsTranslation := itemContentIsTranslatable(item.Content)
			if !contentNeedsTranslation || cache.TranslatedContent != nil {
				return nil
			}
		}
	}

	translator := h.itemTranslator
	if translator == nil {
		translator = newItemTranslator(h.config)
	}

	var translatedTitle *string
	if strings.TrimSpace(item.Title) != "" {
		translated, err := translator.Translate(
			ctx,
			apiKey,
			settings.TranslationModel,
			fmt.Sprintf("Translate the following plain text to %s. Return only the translated text.", settings.TranslationTargetLanguage),
			item.Title,
		)
		if err != nil {
			return fmt.Errorf("translate item title: %w", err)
		}
		translated = strings.TrimSpace(translated)
		if translated != "" {
			translatedTitle = &translated
		}
	}

	var translatedContent *string
	var translatedExcerpt *string
	contentState := classifyHTMLFragment(item.Content)
	if contentState == htmlFragmentValid {
		translatedHTML, err := translateHTMLContent(
			ctx,
			translator,
			apiKey,
			settings.TranslationModel,
			settings.TranslationTargetLanguage,
			item.Content,
		)
		if err != nil {
			return err
		}
		if translatedHTML != "" {
			translatedContent = &translatedHTML
			excerpt := extractPlainTextExcerpt(translatedHTML, 300)
			if excerpt != "" {
				translatedExcerpt = &excerpt
			}
		}
	} else if trimmedContent := strings.TrimSpace(item.Content); trimmedContent != "" && !strings.Contains(trimmedContent, "<") {
		// Plain text content — translate directly without HTML parsing
		translated, err := translator.Translate(
			ctx,
			apiKey,
			settings.TranslationModel,
			fmt.Sprintf("Translate the following plain text to %s. Return only the translated text.", settings.TranslationTargetLanguage),
			trimmedContent,
		)
		if err != nil {
			return fmt.Errorf("translate item content: %w", err)
		}
		translated = strings.TrimSpace(translated)
		if translated != "" {
			translatedContent = &translated
			excerpt := extractPlainTextExcerpt(translated, 300)
			if excerpt != "" {
				translatedExcerpt = &excerpt
			}
		}
	}

	if translatedTitle == nil && translatedContent == nil && translatedExcerpt == nil {
		return errTranslationProducedNoOutput
	}

	if err := h.store.SaveItemTranslation(id, store.SaveItemTranslationInput{
		TranslatedTitle:           translatedTitle,
		TranslatedContent:         translatedContent,
		TranslatedExcerpt:         translatedExcerpt,
		TranslationModel:          settings.TranslationModel,
		TranslationTargetLanguage: settings.TranslationTargetLanguage,
		TranslationUpdatedAt:      time.Now().Unix(),
	}); err != nil {
		if errors.Is(err, store.ErrInvalid) {
			return errTranslationProducedNoOutput
		}
		return err
	}

	return nil
}

func (h *Handler) translateItemsBatch(c *gin.Context) {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequestError(c, "invalid request body")
		return
	}

	if len(req.IDs) == 0 {
		badRequestError(c, "no ids provided")
		return
	}
	if len(req.IDs) > 20 {
		badRequestError(c, "batch size exceeds limit of 20")
		return
	}

	settings, err := h.store.GetTranslationSettings()
	if err != nil {
		internalError(c, err, "get translation settings for batch")
		return
	}

	apiKey, _ := h.resolveTranslationAPIKey(settings.OpenAIAPIKey)
	if apiKey == "" {
		badRequestError(c, "no API key configured")
		return
	}
	if strings.TrimSpace(settings.TranslationModel) == "" {
		badRequestError(c, "no translation model configured")
		return
	}
	if strings.TrimSpace(settings.TranslationTargetLanguage) == "" {
		badRequestError(c, "no target language configured")
		return
	}

	type result struct {
		translated []int64
		failed     []int64
		errors     map[string]string
	}
	res := result{
		translated: make([]int64, 0),
		failed:     make([]int64, 0),
		errors:     make(map[string]string),
	}
	var mu sync.Mutex

	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup

	for _, id := range req.IDs {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			err := h.doTranslateItem(c.Request.Context(), id, settings, apiKey, false)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				res.failed = append(res.failed, id)
				res.errors[strconv.FormatInt(id, 10)] = err.Error()
			} else {
				res.translated = append(res.translated, id)
			}
		}(id)
	}

	wg.Wait()

	dataResponse(c, gin.H{
		"translated": res.translated,
		"failed":     res.failed,
		"errors":     res.errors,
	})
}

type htmlFragmentState int

const (
	htmlFragmentInvalid htmlFragmentState = iota
	htmlFragmentValid
)

func classifyHTMLFragment(content string) htmlFragmentState {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" || !strings.Contains(trimmed, "<") || !strings.Contains(trimmed, ">") {
		return htmlFragmentInvalid
	}

	nodes, err := html.ParseFragment(strings.NewReader(trimmed), &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"})
	if err != nil || !hasElementNode(nodes) {
		return htmlFragmentInvalid
	}

	return htmlFragmentValid
}

func hasElementNode(nodes []*html.Node) bool {
	for _, node := range nodes {
		if containsElementNode(node) {
			return true
		}
	}
	return false
}

func containsElementNode(node *html.Node) bool {
	if node == nil {
		return false
	}
	if node.Type == html.ElementNode {
		return true
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if containsElementNode(child) {
			return true
		}
	}
	return false
}

func hasBalancedHTMLTokens(content string) bool {
	z := html.NewTokenizer(strings.NewReader(content))
	stack := make([]string, 0)
	hasElement := false

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return z.Err() == io.EOF && hasElement && len(stack) == 0
		case html.StartTagToken:
			name, _ := z.TagName()
			tag := strings.ToLower(string(name))
			hasElement = true
			if !isVoidHTMLElement(tag) {
				stack = append(stack, tag)
			}
		case html.SelfClosingTagToken:
			hasElement = true
		case html.EndTagToken:
			name, _ := z.TagName()
			tag := strings.ToLower(string(name))
			if isVoidHTMLElement(tag) {
				continue
			}
			if len(stack) == 0 || stack[len(stack)-1] != tag {
				return false
			}
			stack = stack[:len(stack)-1]
		}
	}
}

func isVoidHTMLElement(tag string) bool {
	switch tag {
	case "area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr":
		return true
	default:
		return false
	}
}

var errTranslatedHTMLInvalid = errors.New("translated html invalid")

func translateHTMLContent(ctx context.Context, translator itemTranslator, apiKey, model, targetLanguage, originalHTML string) (string, error) {
	template, nodes, err := extractTextNodes(originalHTML)
	if err != nil {
		return "", err
	}
	if len(nodes) == 0 {
		return "", nil
	}

	var prompt strings.Builder
	for i, node := range nodes {
		prompt.WriteString(fmt.Sprintf("[%d] %s\n", i, node.Core))
	}

	translatedLines, err := translator.Translate(
		ctx,
		apiKey,
		model,
		fmt.Sprintf("Translate the following numbered HTML text-node contents to %s. Keep the numbering exactly the same. Return only numbered lines.", targetLanguage),
		prompt.String(),
	)
	if err != nil {
		return "", err
	}

	translations, err := parseNumberedTranslations(translatedLines, len(nodes))
	if err != nil {
		return "", errTranslatedHTMLInvalid
	}

	translatedHTML := replaceTextNodes(template, nodes, translations)
	if !validateHTMLStructure(originalHTML, translatedHTML) {
		return "", errTranslatedHTMLInvalid
	}

	return translatedHTML, nil
}

func extractTextNodes(content string) (string, []textNodeInfo, error) {
	nodes, err := html.ParseFragment(strings.NewReader(content), &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"})
	if err != nil {
		return "", nil, fmt.Errorf("parse html fragment: %w", err)
	}

	texts := make([]textNodeInfo, 0)
	index := 0
	for _, node := range nodes {
		collectTextNodes(node, &texts, &index, false)
	}

	var rendered strings.Builder
	for _, node := range nodes {
		if err := html.Render(&rendered, node); err != nil {
			return "", nil, fmt.Errorf("render html fragment: %w", err)
		}
	}

	return rendered.String(), texts, nil
}

func collectTextNodes(node *html.Node, texts *[]textNodeInfo, index *int, skip bool) {
	if node == nil {
		return
	}

	nextSkip := skip
	if node.Type == html.ElementNode {
		tag := strings.ToLower(node.Data)
		if tag == "script" || tag == "style" || tag == "code" || tag == "pre" {
			nextSkip = true
		}
	}

	if node.Type == html.TextNode && !nextSkip {
		leading, core, trailing := splitWhitespace(node.Data)
		if core != "" {
			placeholder := fmt.Sprintf("__FUSION_TRANSLATION_TEXT_%d__", *index)
			*texts = append(*texts, textNodeInfo{
				Placeholder: placeholder,
				Leading:     leading,
				Core:        core,
				Trailing:    trailing,
			})
			node.Data = placeholder
			*index++
		}
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		collectTextNodes(child, texts, index, nextSkip)
	}
}

func splitWhitespace(value string) (string, string, string) {
	start := 0
	for start < len(value) {
		r := value[start]
		if r != ' ' && r != '\n' && r != '\t' && r != '\r' {
			break
		}
		start++
	}
	end := len(value)
	for end > start {
		r := value[end-1]
		if r != ' ' && r != '\n' && r != '\t' && r != '\r' {
			break
		}
		end--
	}
	return value[:start], value[start:end], value[end:]
}

func parseNumberedTranslations(output string, expected int) (map[int]string, error) {
	translations := make(map[int]string, expected)
	lastIndex := -1
	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		index, text, ok := parseNumberedLine(line)
		if ok && index >= 0 && index < expected {
			if strings.TrimSpace(text) == "" {
				return nil, errTranslatedHTMLInvalid
			}
			translations[index] = strings.TrimSpace(text)
			lastIndex = index
		} else if lastIndex >= 0 {
			// Continuation line for a multi-line text node.
			translations[lastIndex] += "\n" + line
		} else {
			return nil, errTranslatedHTMLInvalid
		}
	}
	if len(translations) != expected {
		return nil, errTranslatedHTMLInvalid
	}
	return translations, nil
}

func parseNumberedLine(line string) (int, string, bool) {
	if strings.HasPrefix(line, "[") {
		end := strings.Index(line, "]")
		if end > 1 {
			index, err := strconv.Atoi(line[1:end])
			if err == nil {
				return index, strings.TrimSpace(line[end+1:]), true
			}
		}
	}

	firstSpace := strings.IndexByte(line, ' ')
	if firstSpace <= 0 {
		return 0, "", false
	}

	numberPart := strings.TrimSuffix(line[:firstSpace], ".")
	index, err := strconv.Atoi(numberPart)
	if err != nil {
		return 0, "", false
	}
	return index, strings.TrimSpace(line[firstSpace+1:]), true
}

func replaceTextNodes(template string, nodes []textNodeInfo, translations map[int]string) string {
	result := template
	for i, node := range nodes {
		replacement := node.Leading + translations[i] + node.Trailing
		result = strings.ReplaceAll(result, node.Placeholder, replacement)
	}
	return result
}

func validateHTMLStructure(original, translated string) bool {
	originalNodes, ok := extractStructure(original)
	if !ok {
		return false
	}
	translatedNodes, ok := extractStructure(translated)
	if !ok {
		return false
	}
	if len(originalNodes) != len(translatedNodes) {
		return false
	}
	for i := range originalNodes {
		if originalNodes[i] != translatedNodes[i] {
			return false
		}
	}
	return true
}

func extractStructure(content string) ([]htmlNodeInfo, bool) {
	nodes, err := html.ParseFragment(strings.NewReader(content), &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"})
	if err != nil {
		return nil, false
	}

	result := make([]htmlNodeInfo, 0)
	for _, node := range nodes {
		collectStructure(node, &result)
	}
	return result, true
}

func collectStructure(node *html.Node, result *[]htmlNodeInfo) {
	if node == nil {
		return
	}
	if node.Type == html.ElementNode {
		attrs := make([]string, 0, len(node.Attr))
		for _, attr := range node.Attr {
			attrs = append(attrs, fmt.Sprintf("%s=%q", attr.Key, attr.Val))
		}
		*result = append(*result, htmlNodeInfo{
			Tag:        strings.ToLower(node.Data),
			Attributes: strings.Join(attrs, " "),
		})
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		collectStructure(child, result)
	}
}

// itemContentIsTranslatable reports whether item content should be translated.
// Returns true for valid HTML fragments and plain text (no HTML tags).
// Broken HTML (has '<' but is malformed) returns false.
func itemContentIsTranslatable(content string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "<") {
		return classifyHTMLFragment(trimmed) == htmlFragmentValid
	}
	return true // plain text
}

// extractPlainTextExcerpt strips HTML tags and returns a plain text excerpt of maxLen runes.
// It skips content inside <script> and <style> tags.
func extractPlainTextExcerpt(htmlContent string, maxLen int) string {
	if htmlContent == "" || maxLen <= 0 {
		return ""
	}

	var buf strings.Builder
	z := html.NewTokenizer(strings.NewReader(htmlContent))
	skip := 0

loop:
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			break loop
		case html.StartTagToken:
			name, _ := z.TagName()
			tag := strings.ToLower(string(name))
			if tag == "script" || tag == "style" {
				skip++
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			tag := strings.ToLower(string(name))
			if tag == "script" || tag == "style" {
				skip--
			}
		case html.TextToken:
			if skip <= 0 {
				buf.Write(z.Text())
			}
		}
	}

	// Collapse whitespace and trim. html.NewTokenizer.Text() returns raw text, so we unescape it.
	plainText := html.UnescapeString(strings.Join(strings.Fields(buf.String()), " "))
	runes := []rune(plainText)
	if len(runes) <= maxLen {
		return plainText
	}
	return string(runes[:maxLen])
}
