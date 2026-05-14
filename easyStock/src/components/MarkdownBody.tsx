import ReactMarkdown from "react-markdown";

type Props = {
  markdown: string;
  className?: string;
};

export function MarkdownBody({ markdown, className = "" }: Props) {
  return (
    <div className={`markdown-body ${className}`.trim()}>
      <ReactMarkdown>{markdown}</ReactMarkdown>
    </div>
  );
}
