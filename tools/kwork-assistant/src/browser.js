import fs from "node:fs";
import { chromium } from "playwright-core";

export async function launchKworkBrowser(config) {
  fs.mkdirSync(config.browser.userDataDir, { recursive: true });

  return chromium.launchPersistentContext(config.browser.userDataDir, {
    executablePath: config.browser.executablePath,
    headless: config.browser.headless,
    slowMo: config.browser.slowMoMs,
    locale: "ru-RU",
    viewport: { width: 1440, height: 920 },
  });
}

export async function openPage(context, url, config) {
  const page = await context.newPage();
  await goto(page, url, config);
  return page;
}

export async function goto(page, url, config) {
  await page.goto(url, {
    waitUntil: "domcontentloaded",
    timeout: config.browser.navigationTimeoutMs,
  });
  await page.waitForLoadState("networkidle", { timeout: 7000 }).catch(() => {});
}

export async function pasteReply(context, draft, config, shouldSend) {
  if (!draft.url) {
    throw new Error("Draft does not contain a Kwork URL.");
  }

  const page = await openPage(context, draft.url, config);
  const field = page.locator(config.kwork.selectors.replyField).first();
  await field.waitFor({ state: "visible", timeout: 15000 });

  const tagName = await field.evaluate((node) => node.tagName.toLowerCase());
  const isEditable = await field.evaluate((node) => node.isContentEditable);
  if (tagName === "textarea" || tagName === "input") {
    await field.fill(draft.reply);
  } else if (isEditable) {
    await field.click();
    await page.keyboard.press("Control+A");
    await page.keyboard.insertText(draft.reply);
  } else {
    throw new Error("Reply field is neither an input nor contenteditable.");
  }

  if (!shouldSend) return { sent: false };

  if (config.kwork.selectors.sendButton) {
    await page.locator(config.kwork.selectors.sendButton).first().click();
  } else {
    await page
      .getByRole("button", { name: /отправить|ответить|send|reply/i })
      .first()
      .click();
  }

  return { sent: true };
}
