import crypto from "node:crypto";
import { goto } from "./browser.js";

function normalizeText(text) {
  return text.replace(/\s+/g, " ").trim();
}

function hashText(text) {
  return crypto.createHash("sha256").update(text).digest("hex").slice(0, 16);
}

export async function scanThreadLinks(page, config) {
  const selector = config.kwork.selectors.threadLink;

  const links = await page.locator(selector).evaluateAll((elements) => {
    const seen = new Set();
    return elements
      .map((element) => {
        const anchor = element instanceof HTMLAnchorElement
          ? element
          : element.closest("a");
        if (!anchor) return null;
        const href = new URL(anchor.getAttribute("href") || "", location.href).href;
        if (seen.has(href)) return null;
        seen.add(href);
        return {
          url: href,
          preview: anchor.innerText || anchor.textContent || href,
        };
      })
      .filter(Boolean);
  });

  return links.map((link) => ({
    ...link,
    preview: normalizeText(link.preview || ""),
  }));
}

export async function extractInquiry(context, thread, config) {
  const page = await context.newPage();
  await goto(page, thread.url, config);

  const title = await page.title().catch(() => "");
  const heading = await page
    .locator("h1, h2")
    .first()
    .innerText({ timeout: 5000 })
    .catch(() => "");

  const root = page.locator(config.kwork.selectors.messageRoot).first();
  const text = await root
    .innerText({ timeout: 15000 })
    .catch(() => page.locator("body").innerText({ timeout: 10000 }));

  const attachments = await page.locator("a[href]").evaluateAll((anchors) =>
    anchors
      .map((anchor) => ({
        text: (anchor.textContent || "").trim(),
        url: new URL(anchor.getAttribute("href") || "", location.href).href,
      }))
      .filter((item) =>
        /\.(pdf|docx?|xlsx?|png|jpe?g|webp|zip|rar|7z)(\?|#|$)/i.test(item.url) ||
        /влож|файл|скач|attach|download/i.test(item.text)
      )
  );

  await page.close();

  const threadText = normalizeText(text);
  return {
    url: thread.url,
    preview: thread.preview,
    title: normalizeText(heading || title),
    threadText,
    attachments,
    extractedAt: new Date().toISOString(),
    contentHash: hashText(`${thread.url}\n${threadText}`),
  };
}

