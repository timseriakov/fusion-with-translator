import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeRaw from "rehype-raw";

interface MarkdownRendererProps {
  content: string;
  className?: string;
}

/**
 * MarkdownRenderer component for rendering Markdown content to HTML.
 *
 * Uses react-markdown with:
 * - remark-gfm: GitHub Flavored Markdown support (tables, task lists, etc.)
 * - rehype-raw: Allows rendering raw HTML tags within Markdown
 *
 * @param content - The Markdown string to render
 * @param className - Optional CSS class names to apply to the container
 */
export function MarkdownRenderer({
  content,
  className = "",
}: MarkdownRendererProps) {
  return (
    <div className={className}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeRaw]}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}
