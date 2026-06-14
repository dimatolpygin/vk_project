import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, "..");

function loadDotEnv(filePath) {
  if (!fs.existsSync(filePath)) return;

  const content = fs.readFileSync(filePath, "utf8");
  for (const rawLine of content.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line || line.startsWith("#")) continue;

    const equalsIndex = line.indexOf("=");
    if (equalsIndex === -1) continue;

    const key = line.slice(0, equalsIndex).trim();
    let value = line.slice(equalsIndex + 1).trim();
    if (
      (value.startsWith('"') && value.endsWith('"')) ||
      (value.startsWith("'") && value.endsWith("'"))
    ) {
      value = value.slice(1, -1);
    }
    if (!(key in process.env)) process.env[key] = value;
  }
}

function boolEnv(name, fallback = false) {
  const value = process.env[name];
  if (value == null || value === "") return fallback;
  return ["1", "true", "yes", "on"].includes(value.toLowerCase());
}

function intEnv(name, fallback) {
  const value = Number.parseInt(process.env[name] ?? "", 10);
  return Number.isFinite(value) ? value : fallback;
}

function resolveFromRoot(value, fallback) {
  const raw = value || fallback;
  if (path.isAbsolute(raw)) return raw;
  return path.resolve(rootDir, raw);
}

function splitArgs(value) {
  if (!value) return [];

  const args = [];
  let current = "";
  let quote = null;

  for (let index = 0; index < value.length; index += 1) {
    const char = value[index];
    if (quote) {
      if (char === "\\" && value[index + 1] === quote) {
        current += quote;
        index += 1;
        continue;
      }
      if (char === quote) quote = null;
      else current += char;
      continue;
    }

    if (char === '"' || char === "'") {
      quote = char;
      continue;
    }

    if (/\s/.test(char)) {
      if (current) {
        args.push(current);
        current = "";
      }
      continue;
    }

    current += char;
  }

  if (current) args.push(current);
  return args;
}

export function loadConfig() {
  loadDotEnv(path.resolve(rootDir, ".env"));

  const defaultChromium =
    "C:\\Users\\GigaChat\\AppData\\Local\\ms-playwright\\chromium-1208\\chrome-win64\\chrome.exe";

  return {
    rootDir,
    browser: {
      executablePath:
        process.env.CHROMIUM_EXECUTABLE_PATH || defaultChromium,
      userDataDir: resolveFromRoot(process.env.KWORK_USER_DATA_DIR, "profile"),
      headless: boolEnv("KWORK_HEADLESS", false),
      slowMoMs: intEnv("KWORK_SLOW_MO_MS", 0),
      navigationTimeoutMs: intEnv("KWORK_NAVIGATION_TIMEOUT_MS", 45000),
    },
    kwork: {
      inboxUrl: process.env.KWORK_INBOX_URL || "https://kwork.ru/inbox",
      loginUrl: process.env.KWORK_LOGIN_URL || "https://kwork.ru/login",
      pollMs: intEnv("KWORK_POLL_MS", 60000),
      maxThreadsPerRun: intEnv("KWORK_MAX_THREADS_PER_RUN", 3),
      autoSend: boolEnv("KWORK_AUTO_SEND", false),
      selectors: {
        threadLink:
          process.env.KWORK_THREAD_LINK_SELECTOR ||
          'a[href*="/inbox"], a[href*="/orders"], a[href*="/projects"]',
        messageRoot:
          process.env.KWORK_MESSAGE_ROOT_SELECTOR || "main, body",
        replyField:
          process.env.KWORK_REPLY_FIELD_SELECTOR ||
          'textarea, [contenteditable="true"]',
        sendButton: process.env.KWORK_SEND_BUTTON_SELECTOR || "",
      },
    },
    data: {
      stateFile: resolveFromRoot(process.env.KWORK_STATE_FILE, "data/state.json"),
      draftsDir: resolveFromRoot(process.env.KWORK_DRAFTS_DIR, "data/drafts"),
      profileFile: resolveFromRoot(process.env.KWORK_PROFILE_FILE, "data/profile.md"),
      examplesFile: resolveFromRoot(process.env.KWORK_EXAMPLES_FILE, "data/examples.md"),
    },
    ai: {
      engine: (process.env.AI_ENGINE || "pi").toLowerCase(),
      piCommand: process.env.PI_COMMAND || "pi",
      piArgs: splitArgs(process.env.PI_ARGS || "--mode rpc --no-session"),
      piTimeoutMs: intEnv("PI_TIMEOUT_MS", 240000),
      openaiBaseUrl: process.env.OPENAI_BASE_URL || "https://api.openai.com/v1",
      openaiApiKey: process.env.OPENAI_API_KEY || "",
      openaiModel: process.env.OPENAI_MODEL || "gpt-4.1-mini",
    },
    telegram: {
      botToken: process.env.TELEGRAM_BOT_TOKEN || "",
      chatId: process.env.TELEGRAM_CHAT_ID || "",
    },
  };
}
