import { spawn } from "node:child_process";

export async function askPi(prompt, config) {
  return new Promise((resolve, reject) => {
    const child = spawn(config.ai.piCommand, config.ai.piArgs, {
      stdio: ["pipe", "pipe", "pipe"],
      windowsHide: true,
    });

    const timeout = setTimeout(() => {
      child.kill();
      reject(new Error(`Pi RPC timed out after ${config.ai.piTimeoutMs} ms.`));
    }, config.ai.piTimeoutMs);

    const pendingResponses = new Map();
    const eventWaiters = [];
    let stdoutBuffer = "";
    let stderr = "";
    let settled = false;

    function finish(error, value) {
      if (settled) return;
      settled = true;
      clearTimeout(timeout);
      child.kill();
      if (error) reject(error);
      else resolve(value);
    }

    function send(command) {
      child.stdin.write(`${JSON.stringify(command)}\n`);
    }

    function waitForResponse(id) {
      return new Promise((res, rej) => {
        pendingResponses.set(id, { res, rej });
      });
    }

    function waitForEvent(predicate) {
      return new Promise((res) => {
        eventWaiters.push({ predicate, res });
      });
    }

    function handleMessage(message) {
      if (message.type === "response" && message.id && pendingResponses.has(message.id)) {
        const waiter = pendingResponses.get(message.id);
        pendingResponses.delete(message.id);
        if (message.success === false) {
          waiter.rej(new Error(`Pi RPC command failed: ${JSON.stringify(message)}`));
        } else {
          waiter.res(message);
        }
        return;
      }

      for (let index = 0; index < eventWaiters.length; index += 1) {
        const waiter = eventWaiters[index];
        if (waiter.predicate(message)) {
          eventWaiters.splice(index, 1);
          waiter.res(message);
          index -= 1;
        }
      }
    }

    child.on("error", (error) => {
      finish(new Error(`Unable to start Pi command "${config.ai.piCommand}": ${error.message}`));
    });

    child.stderr.on("data", (chunk) => {
      stderr += chunk.toString("utf8");
    });

    child.stdout.on("data", (chunk) => {
      stdoutBuffer += chunk.toString("utf8");
      let newlineIndex;
      while ((newlineIndex = stdoutBuffer.indexOf("\n")) !== -1) {
        const rawLine = stdoutBuffer.slice(0, newlineIndex).replace(/\r$/, "");
        stdoutBuffer = stdoutBuffer.slice(newlineIndex + 1);
        if (!rawLine.trim()) continue;

        try {
          handleMessage(JSON.parse(rawLine));
        } catch {
          // Ignore non-JSON startup noise from wrappers.
        }
      }
    });

    child.on("exit", (code) => {
      if (!settled && code !== 0) {
        finish(new Error(`Pi RPC exited with code ${code}. ${stderr}`.trim()));
      }
    });

    (async () => {
      try {
        const promptId = `prompt-${Date.now()}`;
        const agentDone = waitForEvent((message) => message.type === "agent_end");
        send({ id: promptId, type: "prompt", message: prompt });
        await waitForResponse(promptId);
        await agentDone;

        const lastId = `last-${Date.now()}`;
        send({ id: lastId, type: "get_last_assistant_text" });
        const response = await waitForResponse(lastId);
        const text = response.data?.text;
        if (!text) throw new Error("Pi RPC returned no assistant text.");
        finish(null, text);
      } catch (error) {
        finish(error);
      }
    })();
  });
}

