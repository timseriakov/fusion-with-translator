package md

import (
	"strings"
	"testing"
)

func TestFromHTML(t *testing.T) {
	tests := []struct {
		name      string
		html      string
		expected  string
		expectErr bool
	}{
		{
			name:     "Basic paragraph",
			html:     "<p>Hello, world!</p>",
			expected: "Hello, world!",
		},
		{
			name:     "Basic link",
			html:     `<a href="https://example.com">Example</a>`,
			expected: "[Example](https://example.com)",
		},
		{
			name:     "Unordered list",
			html:     "<ul><li>Item 1</li><li>Item 2</li></ul>",
			expected: "* Item 1\n* Item 2",
		},
		{
			name:     "Iframe preservation",
			html:     `<p>Some text</p><iframe src="https://www.youtube.com/embed/dQw4w9WgXcQ" width="560" height="315" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe><p>More text</p>`,
			expected: "Some text\n\n<iframe src=\"https://www.youtube.com/embed/dQw4w9WgXcQ\" width=\"560\" height=\"315\" frameborder=\"0\" allow=\"accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture\" allowfullscreen=\"\"></iframe>\n\nMore text",
		},
		{
			name:     "Iframe with minimal attributes",
			html:     `<iframe></iframe>`,
			expected: "<iframe></iframe>",
		},
		{
			name:     "Malformed HTML - unclosed tag",
			html:     "<p>Hello, world!",
			expected: "Hello, world!",
		},
		{
			name:     "HTML with mixed content",
			html:     "<div><p>Paragraph 1</p><ul><li>List 1</li></ul></div>",
			expected: "Paragraph 1\n\n* List 1",
		},
		{
			name:     "Empty HTML",
			html:     "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			markdown, err := FromHTML(tt.html)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected an error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if strings.TrimSpace(tt.expected) != strings.TrimSpace(markdown) {
					t.Errorf("got %q, want %q", markdown, tt.expected)
				}
			}
		})
	}
}
