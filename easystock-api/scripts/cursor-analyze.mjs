/**
 * Bridge script: Go backend calls this via subprocess to use Cursor SDK for AI analysis.
 *
 * Usage:   node cursor-analyze.mjs <input.json>
 * Input:   { "system": "...", "user": "...", "model": "claude-sonnet-4-6" }
 * Output:  AI response text to stdout
 * Env:     CURSOR_API_KEY required
 */
import { readFileSync } from "fs";
import { Agent } from "@cursor/sdk";

const inputPath = process.argv[2];
if (!inputPath) {
  process.stderr.write("usage: node cursor-analyze.mjs <input.json>\n");
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
  local: { cwd: process.cwd() },
});

try {
  const run = await agent.send(prompt);
  let result = "";

  for await (const event of run.stream()) {
    if (event.type === "assistant" && event.message?.content) {
      for (const block of event.message.content) {
        if (block.type === "text") {
          result += block.text;
        }
      }
    }
  }

  // Fallback: check run.result if stream didn't capture text
  if (!result && run.result) {
    result = run.result;
  }

  process.stdout.write(result);
} finally {
  agent.close();
}
