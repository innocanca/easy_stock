/**
 * Bridge script: Go backend calls this via subprocess to use Cursor SDK for AI analysis.
 *
 * Usage:   node cursor-analyze.mjs <input.json> [--stream]
 * Input:   { "system": "...", "user": "...", "model": "claude-sonnet-4-6" }
 * Output:  AI response text to stdout (--stream: writes chunks as they arrive)
 * Env:     CURSOR_API_KEY required
 */
import { readFileSync } from "fs";
import { Agent } from "@cursor/sdk";

const args = process.argv.slice(2);
const streamMode = args.includes("--stream");
const inputPath = args.find((a) => !a.startsWith("--"));

if (!inputPath) {
  process.stderr.write("usage: node cursor-analyze.mjs <input.json> [--stream]\n");
  process.exit(1);
}

const apiKey = process.env.CURSOR_API_KEY;
if (!apiKey) {
  process.stderr.write("CURSOR_API_KEY environment variable is required\n");
  process.exit(1);
}

const input = JSON.parse(readFileSync(inputPath, "utf-8"));
const prompt = input.system ? `${input.system}\n\n${input.user}` : input.user;
const modelId = input.model || "claude-sonnet-4-6";

const agent = await Agent.create({
  apiKey,
  model: { id: modelId },
  // Use cloud runtime to avoid local-runtime prerequisites (e.g. ripgrep).
  // This bridge uses Cursor as an LLM provider, not as a codebase tool runner.
  cloud: { repos: [] },
});

try {
  const run = await agent.send(prompt);
  let result = "";

  for await (const event of run.stream()) {
    if (event.type === "assistant" && event.message?.content) {
      for (const block of event.message.content) {
        if (block.type === "text") {
          if (streamMode) {
            process.stdout.write(block.text);
          } else {
            result += block.text;
          }
        }
      }
    }
  }

  if (!streamMode) {
    if (!result && run.result) {
      result = run.result;
    }
    process.stdout.write(result);
  }
} finally {
  // Prefer SDK dispose; fall back to legacy close if present.
  if (agent?.[Symbol.asyncDispose]) {
    await agent[Symbol.asyncDispose]();
  } else if (agent?.close) {
    agent.close();
  }
}
