import path from "node:path";

export async function sendTelegramDraft(config, inquiry, draft, draftPath) {
  if (!config.telegram.botToken || !config.telegram.chatId) return;

  const text = [
    "Новый черновик ответа Kwork",
    "",
    `Заявка: ${inquiry.url}`,
    `Файл: ${path.basename(draftPath)}`,
    `Confidence: ${draft.confidence}`,
    "",
    "Ответ:",
    draft.reply,
  ].join("\n");

  const response = await fetch(
    `https://api.telegram.org/bot${config.telegram.botToken}/sendMessage`,
    {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        chat_id: config.telegram.chatId,
        text: text.slice(0, 3900),
        disable_web_page_preview: true,
      }),
    }
  );

  if (!response.ok) {
    const body = await response.text();
    throw new Error(`Telegram send failed: ${response.status} ${body}`);
  }
}

