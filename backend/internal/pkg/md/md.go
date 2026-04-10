package md

import (
	"fmt"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
)

// FromHTML converts HTML to Markdown, preserving iframes.
func FromHTML(htmlString string) (string, error) {
	conv := converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(
				commonmark.WithBulletListMarker("*"),
			),
		),
	)

	// Preserve iframe tags by rendering them as HTML.
	// We use PriorityEarly to override the default "remove" behavior from the base plugin.
	conv.Register.RendererFor("iframe", converter.TagTypeBlock, base.RenderAsHTML, converter.PriorityEarly)

	markdown, err := conv.ConvertString(htmlString)
	if err != nil {
		return "", fmt.Errorf("failed to convert HTML to Markdown: %w", err)
	}

	return markdown, nil
}
