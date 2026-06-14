import fs from "node:fs";
import path from "node:path";

export function loadState(filePath) {
  if (!fs.existsSync(filePath)) {
    return { seenThreads: {}, processedHashes: {} };
  }

  const state = JSON.parse(fs.readFileSync(filePath, "utf8"));
  return {
    seenThreads: state.seenThreads || {},
    processedHashes: state.processedHashes || {},
  };
}

export function saveState(filePath, state) {
  fs.mkdirSync(path.dirname(filePath), { recursive: true });
  fs.writeFileSync(filePath, JSON.stringify(state, null, 2), "utf8");
}

export function markProcessed(state, inquiry, draftPath) {
  state.seenThreads[inquiry.url] = {
    firstSeenAt: state.seenThreads[inquiry.url]?.firstSeenAt || inquiry.extractedAt,
    lastProcessedAt: new Date().toISOString(),
    contentHash: inquiry.contentHash,
    draftPath,
  };
  state.processedHashes[inquiry.contentHash] = {
    url: inquiry.url,
    processedAt: new Date().toISOString(),
    draftPath,
  };
}

export function isProcessed(state, item) {
  return Boolean(state.seenThreads[item.url]);
}

