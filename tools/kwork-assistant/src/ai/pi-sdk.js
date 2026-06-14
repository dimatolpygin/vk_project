import fs from "node:fs";
import path from "node:path";
import {
  AuthStorage,
  DefaultResourceLoader,
  ModelRegistry,
  SessionManager,
  createAgentSession,
  getAgentDir,
} from "@earendil-works/pi-coding-agent";

let runtimePromise;

export async function askPi(prompt, config) {
  const runtime = await getRuntime(config);
  await runtime.loader.reload().catch((error) => {
    console.warn(`Pi loader reload failed: ${error?.message || error}`);
  });

  const { session } = await createAgentSession({
    sessionManager: SessionManager.inMemory(),
    authStorage: runtime.authStorage,
    modelRegistry: runtime.modelRegistry,
    model: runtime.model,
    thinkingLevel: config.ai.thinkingLevel,
    resourceLoader: runtime.loader,
    noTools: "all",
  });

  let text = "";
  session.subscribe((event) => {
    if (
      event.type === "message_update" &&
      event.assistantMessageEvent?.type === "text_delta"
    ) {
      text += event.assistantMessageEvent.delta;
    }
  });

  let timedOut = false;
  const guard = setTimeout(() => {
    timedOut = true;
    session.abort().catch(() => {});
  }, config.ai.piTimeoutMs);

  try {
    await session.prompt(prompt);
  } finally {
    clearTimeout(guard);
    session.dispose();
  }

  if (timedOut) {
    throw new Error(`Pi agent timed out after ${config.ai.piTimeoutMs} ms.`);
  }

  text = text.trim();
  if (!text) {
    throw new Error("Pi agent returned empty text.");
  }
  return text;
}

async function getRuntime(config) {
  runtimePromise ||= createRuntime(config);
  return runtimePromise;
}

async function createRuntime(config) {
  const authStorage = AuthStorage.create();
  const modelRegistry = ModelRegistry.create(authStorage);
  const available = await modelRegistry.getAvailable();

  if (available.length === 0) {
    throw new Error(
      "Pi has no available models. Start `pi` in a terminal and run /login, then restart Kwork assistant."
    );
  }

  const model = pickModel(available, modelRegistry, config.ai.model);
  console.log(
    `Pi SDK model: ${model.provider}/${model.id}, thinking: ${config.ai.thinkingLevel}`
  );

  const piDir = path.join(config.rootDir, ".pi");
  const loader = new DefaultResourceLoader({
    cwd: config.rootDir,
    agentDir: getAgentDir(),
    skillsOverride: (current) => ({
      skills: current.skills.filter((skill) =>
        (skill.filePath || "").startsWith(piDir)
      ),
      diagnostics: current.diagnostics,
    }),
    agentsFilesOverride: (current) => ({
      agentsFiles: [
        ...current.agentsFiles,
        ...readContextFiles(config),
      ],
    }),
  });
  await loader.reload();

  const { skills } = loader.getSkills();
  console.log(
    `Pi SDK skills: ${skills.length ? skills.map((skill) => skill.name).join(", ") : "(none)"}`
  );

  return { authStorage, modelRegistry, model, loader };
}

function pickModel(available, modelRegistry, requested) {
  if (requested) {
    const [provider, id] = requested.split("/");
    const found =
      available.find((model) => model.provider === provider && model.id === id) ||
      modelRegistry.find(provider, id);
    if (found) return found;
    console.warn(`PI_MODEL ${requested} not found. Falling back to available model.`);
  }

  return (
    available.find((model) => model.id === "gpt-5.4") ||
    available.find((model) => model.id === "gpt-5.5") ||
    available.find((model) => !/spark|mini/i.test(model.id)) ||
    available[0]
  );
}

function readContextFiles(config) {
  const files = [];
  for (const filePath of [config.data.profileFile, config.data.examplesFile]) {
    try {
      const content = fs.readFileSync(filePath, "utf8").trim();
      if (content) files.push({ path: filePath, content });
    } catch {
      // Knowledge files are created by the caller before Pi is invoked.
    }
  }
  return files;
}
