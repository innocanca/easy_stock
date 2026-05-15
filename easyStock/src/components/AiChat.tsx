import { useRef, useState } from "react";
import { getApiBase } from "@/api/client";
import { MarkdownBody } from "./MarkdownBody";

interface Message {
  role: "user" | "assistant";
  content: string;
}

export function AiChat({
  stockCode,
  stockName,
}: {
  stockCode: string;
  stockName: string;
}) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [streaming, setStreaming] = useState(false);
  const bottomRef = useRef<HTMLDivElement>(null);

  const scrollToBottom = () => {
    setTimeout(() => bottomRef.current?.scrollIntoView({ behavior: "smooth" }), 50);
  };

  const send = async () => {
    const q = input.trim();
    if (!q || streaming) return;
    setInput("");
    setMessages((prev) => [...prev, { role: "user", content: q }]);
    setStreaming(true);

    let assistantContent = "";
    setMessages((prev) => [...prev, { role: "assistant", content: "" }]);

    try {
      const base = getApiBase();
      const res = await fetch(`${base}/api/chat`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          stock_code: stockCode,
          stock_name: stockName,
          question: q,
        }),
      });

      if (!res.ok) {
        const text = await res.text().catch(() => "");
        throw new Error(text || `HTTP ${res.status}`);
      }

      const reader = res.body?.getReader();
      if (!reader) throw new Error("No response body");

      const dec = new TextDecoder();
      let buf = "";

      const handleBlock = (block: string) => {
        const lines = block.split(/\r?\n/).filter((l) => l.length > 0);
        let event = "";
        const dataLines: string[] = [];
        for (const line of lines) {
          if (line.startsWith("event:")) event = line.slice(6).trim();
          else if (line.startsWith("data:")) dataLines.push(line.slice(5).trimStart());
        }
        if (dataLines.length === 0) return;
        const data = dataLines.join("\n");

        if (event === "chunk") {
          try {
            const text = JSON.parse(data) as string;
            assistantContent += text;
            setMessages((prev) => {
              const copy = [...prev];
              copy[copy.length - 1] = { role: "assistant", content: assistantContent };
              return copy;
            });
            scrollToBottom();
          } catch { /* skip */ }
        } else if (event === "error") {
          try {
            const errText = JSON.parse(data) as string;
            assistantContent += `\n\n_错误: ${errText}_`;
            setMessages((prev) => {
              const copy = [...prev];
              copy[copy.length - 1] = { role: "assistant", content: assistantContent };
              return copy;
            });
          } catch { /* skip */ }
        }
      };

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buf += dec.decode(value, { stream: true });
        const parts = buf.split("\n\n");
        buf = parts.pop() ?? "";
        for (const part of parts) handleBlock(part);
      }
      if (buf.trim()) handleBlock(buf);
    } catch (e: unknown) {
      const errMsg = e instanceof Error ? e.message : "请求失败";
      setMessages((prev) => {
        const copy = [...prev];
        copy[copy.length - 1] = { role: "assistant", content: `_错误: ${errMsg}_` };
        return copy;
      });
    } finally {
      setStreaming(false);
      scrollToBottom();
    }
  };

  return (
    <div className="ai-chat">
      <div className="ai-chat-messages">
        {messages.length === 0 && (
          <div className="ai-chat-empty">
            <p>向 AI 提问关于 <strong>{stockName || stockCode}</strong> 的投资问题</p>
            <div className="ai-chat-suggestions">
              {[
                `${stockName}现在估值合理吗？`,
                `分析一下${stockName}的盈利能力趋势`,
                `${stockName}的核心竞争优势是什么？`,
              ].map((s) => (
                <button
                  key={s}
                  className="ai-chat-suggestion"
                  onClick={() => { setInput(s); }}
                >
                  {s}
                </button>
              ))}
            </div>
          </div>
        )}
        {messages.map((msg, i) => (
          <div key={i} className={`ai-chat-msg ai-chat-msg--${msg.role}`}>
            <div className="ai-chat-msg-label">
              {msg.role === "user" ? "你" : "AI 分析师"}
            </div>
            <div className="ai-chat-msg-body">
              {msg.role === "assistant" ? (
                <MarkdownBody markdown={msg.content || "思考中…"} />
              ) : (
                <p>{msg.content}</p>
              )}
            </div>
          </div>
        ))}
        <div ref={bottomRef} />
      </div>
      <div className="ai-chat-input-row">
        <input
          type="text"
          className="ai-chat-input"
          placeholder="输入你的问题…"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => { if (e.key === "Enter") send(); }}
          disabled={streaming}
        />
        <button
          className="report-btn report-btn--primary"
          onClick={send}
          disabled={streaming || !input.trim()}
        >
          {streaming ? "回答中…" : "发送"}
        </button>
      </div>
    </div>
  );
}
