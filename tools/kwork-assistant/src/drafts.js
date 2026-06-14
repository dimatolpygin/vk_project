import fs from "node:fs";
import path from "node:path";
import { askPi } from "./ai/pi-sdk.js";

function readOptional(filePath, fallback) {
  if (!fs.existsSync(filePath)) return fallback;
  return fs.readFileSync(filePath, "utf8").trim();
}

function extractJson(text) {
  const first = text.indexOf("{");
  const last = text.lastIndexOf("}");
  if (first === -1 || last === -1 || last <= first) {
    throw new Error("AI response does not contain JSON.");
  }
  return JSON.parse(text.slice(first, last + 1));
}

export function loadKnowledge(config) {
  fs.mkdirSync(path.dirname(config.data.profileFile), { recursive: true });

  const profile = readOptional(
    config.data.profileFile,
    "Seller profile is not filled yet. Ask concise clarifying questions and avoid exact prices."
  );
  const examples = readOptional(
    config.data.examplesFile,
    "No examples provided yet. Use a polite, practical, human tone."
  );

  return { profile, examples };
}

export async function generateDraft(inquiry, config) {
  const knowledge = loadKnowledge(config);
  const prompt = buildPrompt(inquiry, knowledge);

  const raw = await askPi(prompt, config);
  const parsed = extractJson(raw);
  return normalizeDraft(parsed, raw);
}

function normalizeDraft(parsed, raw) {
  const reply = String(parsed.reply || "").trim();
  if (!reply) throw new Error("AI response JSON has no reply field.");

  return {
    analysis: String(parsed.analysis || "").trim(),
    reply,
    confidence: Number(parsed.confidence || 0),
    questions: Array.isArray(parsed.questions)
      ? parsed.questions.map(String)
      : [],
    risks: Array.isArray(parsed.risks)
      ? parsed.risks.map(String)
      : [],
    shouldReply: parsed.shouldReply !== false,
    raw,
  };
}

function buildPrompt(inquiry, knowledge) {
  return `You are a Russian-speaking sales assistant for a freelancer on Kwork.

Task: study a new Kwork request/dialog and draft a natural human reply in the seller's style.

Rules:
- Do not mention that AI wrote the reply.
- Do not invent portfolio facts, prices, guarantees, or deadlines.
- If the brief is vague, ask 2-4 concrete clarifying questions.
- Sound like a calm expert, not a template.
- Prefer short paragraphs.
- If the request is risky or irrelevant, still draft a polite reply with clarification or refusal.
- Return ONLY valid JSON, no markdown fences.

JSON schema:
{
  "analysis": "short private analysis for seller in Russian",
  "reply": "message to send to the client in Russian",
  "questions": ["clarifying question"],
  "risks": ["risk or missing info"],
  "confidence": 0.0,
  "shouldReply": true
}

Seller profile:
${knowledge.profile}

Successful examples and portfolio notes:
${knowledge.examples}

Kwork request/dialog:
URL: ${inquiry.url}
Title: ${inquiry.title}
Preview: ${inquiry.preview}
Attachments: ${JSON.stringify(inquiry.attachments, null, 2)}
Text:
${inquiry.threadText}
`;
}

export function saveDraft(inquiry, draft, config) {
  fs.mkdirSync(config.data.draftsDir, { recursive: true });

  const safeDate = new Date().toISOString().replace(/[:.]/g, "-");
  const filePath = path.join(
    config.data.draftsDir,
    `${safeDate}-${inquiry.contentHash}.md`
  );

  const content = `---
url: ${inquiry.url}
contentHash: ${inquiry.contentHash}
createdAt: ${new Date().toISOString()}
confidence: ${draft.confidence}
shouldReply: ${draft.shouldReply}
---

# Kwork Draft

## Analysis

${draft.analysis || "_No analysis returned._"}

## Risks

${draft.risks.length ? draft.risks.map((risk) => `- ${risk}`).join("\n") : "- None"}

## Questions

${draft.questions.length ? draft.questions.map((question) => `- ${question}`).join("\n") : "- None"}

## Reply

<!-- KWORK_REPLY_START -->
${draft.reply}
<!-- KWORK_REPLY_END -->

## Source

${inquiry.threadText}
`;

  fs.writeFileSync(filePath, content, "utf8");
  return filePath;
}

export function readDraft(filePath) {
  const content = fs.readFileSync(filePath, "utf8");
  const url = content.match(/^url:\s*(.+)$/m)?.[1]?.trim() || "";
  const reply =
    content.match(/<!-- KWORK_REPLY_START -->([\s\S]*?)<!-- KWORK_REPLY_END -->/)?.[1]?.trim() ||
    "";

  if (!reply) {
    throw new Error(`No reply marker found in draft: ${filePath}`);
  }

  return { filePath, url, reply };
}
