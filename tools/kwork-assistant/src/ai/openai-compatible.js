export async function askOpenAICompatible(prompt, config) {
  if (!config.ai.openaiApiKey) {
    throw new Error("OPENAI_API_KEY is required when AI_ENGINE=openai.");
  }

  const baseUrl = config.ai.openaiBaseUrl.replace(/\/+$/, "");
  const response = await fetch(`${baseUrl}/chat/completions`, {
    method: "POST",
    headers: {
      authorization: `Bearer ${config.ai.openaiApiKey}`,
      "content-type": "application/json",
    },
    body: JSON.stringify({
      model: config.ai.openaiModel,
      temperature: 0.4,
      messages: [
        {
          role: "system",
          content:
            "You draft careful, natural Russian replies for a freelancer. Return strict JSON only.",
        },
        { role: "user", content: prompt },
      ],
    }),
  });

  if (!response.ok) {
    const body = await response.text();
    throw new Error(`OpenAI-compatible request failed: ${response.status} ${body}`);
  }

  const payload = await response.json();
  const text = payload.choices?.[0]?.message?.content;
  if (!text) throw new Error("OpenAI-compatible response has no content.");
  return text;
}

