import fs from "node:fs";
import path from "node:path";
import { launchKworkBrowser, openPage, pasteReply } from "./browser.js";
import { loadConfig } from "./config.js";
import { generateDraft, readDraft, saveDraft } from "./drafts.js";
import { extractInquiry, scanThreadLinks } from "./extract.js";
import { isProcessed, loadState, markProcessed, saveState } from "./state.js";
import { sendTelegramDraft } from "./telegram.js";

const command = process.argv[2] || "help";

async function main() {
  const config = loadConfig();

  if (command === "help" || command === "--help" || command === "-h") {
    printHelp();
    return;
  }

  if (command === "login") {
    await runLogin(config);
    return;
  }

  if (command === "once") {
    const context = await launchKworkBrowser(config);
    try {
      await processOnce(context, config);
    } finally {
      await context.close();
    }
    return;
  }

  if (command === "prime") {
    const context = await launchKworkBrowser(config);
    try {
      await primeSeenThreads(context, config);
    } finally {
      await context.close();
    }
    return;
  }

  if (command === "monitor") {
    await runMonitor(config);
    return;
  }

  if (command === "reply") {
    const draftArg = process.argv[3];
    if (!draftArg) throw new Error("Usage: npm run reply -- <draft-file> [--send]");
    const shouldSend = process.argv.includes("--send");
    const draft = readDraft(path.resolve(draftArg));

    const context = await launchKworkBrowser(config);
    try {
      const result = await pasteReply(context, draft, config, shouldSend);
      console.log(result.sent ? "Reply sent." : "Reply pasted, not sent.");
    } finally {
      if (!shouldSend) {
        console.log("Press Enter to close the browser.");
        await waitForEnter();
      }
      await context.close();
    }
    return;
  }

  throw new Error(`Unknown command: ${command}`);
}

async function runLogin(config) {
  const context = await launchKworkBrowser(config);
  try {
    await openPage(context, config.kwork.loginUrl || config.kwork.inboxUrl, config);
    console.log("Log in to Kwork in the opened Chromium window.");
    console.log("Press Enter here after the seller account is authorized.");
    await waitForEnter();
  } finally {
    await context.close();
  }
}

async function runMonitor(config) {
  const context = await launchKworkBrowser(config);
  try {
    while (true) {
      await processOnce(context, config).catch((error) => {
        console.error(`[${new Date().toISOString()}] ${error.stack || error.message}`);
      });
      await sleep(config.kwork.pollMs);
    }
  } finally {
    await context.close();
  }
}

async function processOnce(context, config) {
  ensureKnowledgeFiles(config);

  const state = loadState(config.data.stateFile);
  const page = await openPage(context, config.kwork.inboxUrl, config);
  const threads = await scanThreadLinks(page, config);
  await page.close();

  const freshThreads = threads
    .filter((thread) => !isProcessed(state, thread))
    .slice(0, config.kwork.maxThreadsPerRun);

  console.log(`Found ${threads.length} thread links, ${freshThreads.length} new.`);
  if (!freshThreads.length) return;

  for (const thread of freshThreads) {
    console.log(`Processing: ${thread.url}`);
    const inquiry = await extractInquiry(context, thread, config);

    if (state.processedHashes[inquiry.contentHash]) {
      markProcessed(state, inquiry, state.processedHashes[inquiry.contentHash].draftPath);
      saveState(config.data.stateFile, state);
      continue;
    }

    const draft = await generateDraft(inquiry, config);
    const draftPath = saveDraft(inquiry, draft, config);
    markProcessed(state, inquiry, draftPath);
    saveState(config.data.stateFile, state);

    console.log(`Draft saved: ${draftPath}`);
    await sendTelegramDraft(config, inquiry, draft, draftPath).catch((error) => {
      console.error(`Telegram notification failed: ${error.message}`);
    });

    if (config.kwork.autoSend && draft.shouldReply) {
      await pasteReply(context, { url: inquiry.url, reply: draft.reply }, config, true);
      console.log("Auto-sent reply.");
    }
  }
}

async function primeSeenThreads(context, config) {
  const state = loadState(config.data.stateFile);
  const page = await openPage(context, config.kwork.inboxUrl, config);
  const threads = await scanThreadLinks(page, config);
  await page.close();

  const now = new Date().toISOString();
  for (const thread of threads) {
    state.seenThreads[thread.url] ||= {
      firstSeenAt: now,
      lastProcessedAt: now,
      contentHash: "",
      draftPath: "",
      primed: true,
    };
  }

  saveState(config.data.stateFile, state);
  console.log(`Primed ${threads.length} visible thread links.`);
}

function ensureKnowledgeFiles(config) {
  const profileExample = path.join(config.rootDir, "data/profile.example.md");
  const examplesExample = path.join(config.rootDir, "data/examples.example.md");

  for (const [target, example] of [
    [config.data.profileFile, profileExample],
    [config.data.examplesFile, examplesExample],
  ]) {
    if (fs.existsSync(target)) continue;
    fs.mkdirSync(path.dirname(target), { recursive: true });
    fs.copyFileSync(example, target);
  }
}

function waitForEnter() {
  return new Promise((resolve) => {
    process.stdin.resume();
    process.stdin.once("data", () => resolve());
  });
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function printHelp() {
  console.log(`Kwork assistant

Commands:
  npm run login
  npm run prime
  npm run once
  npm run monitor
  npm run reply -- <draft-file> [--send]
`);
}

main().catch((error) => {
  console.error(error.stack || error.message);
  process.exitCode = 1;
});
