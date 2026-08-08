# Seedance 2.0 — Image-to-Video API (WaveSpeed)

**Модель:** `bytedance/seedance-2.0/image-to-video`

Генерирует кинематографичные видео из референсного изображения и текстового промта. Нативная синхронизация аудио и видео, управление камерой и светом на уровне режиссёра, высокая стабильность движения. Построена на унифицированной мультимодальной архитектуре Seed: сохраняет объект и композицию входного изображения, добавляя выразительное и физически корректное движение.

```
POST https://api.wavespeed.ai/api/v3/bytedance/seedance-2.0/image-to-video
```

---

## Аутентификация

Bearer-токен. Ключ хранить в серверной переменной окружения — **никогда** не встраивать в браузерный или мобильный код.

```bash
export WAVESPEED_API_KEY="your-api-key"
```

Заголовок запроса:

```
Authorization: Bearer ${WAVESPEED_API_KEY}
Content-Type: application/json
```

---

## Схема работы

Генерация асинхронная, в три шага:

| Шаг | Действие |
|---|---|
| 1. Submit | `POST` с JSON-телом. Сохранить `id` и `urls.get` из ответа. |
| 2. Poll | Опрашивать тот же `urls.get`. Начинать с ~2 сек, для долгих задач увеличивать интервал до ~10 сек. |
| 3. Read | При `status: completed` прекратить опрос и забрать `data.outputs` из этого же ответа. |

> URL из `urls.get` — одновременно и статус-эндпоинт, и эндпоинт результата. Ответ со статусом `completed` уже содержит `outputs`, дополнительный запрос не нужен.

**Статусы задачи:** `created` · `processing` · `completed` · `failed` · `cancelled` · `timeout`

Все REST-ответы приходят в конверте `{ code, message, data }`. Поля предсказания (`status`, `outputs` и т.д.) лежат внутри `data`.

---

## Входные параметры

| Параметр | Тип | По умолчанию | Значения / диапазон | Описание |
|---|---|---|---|---|
| `prompt` **(обязательный)** | string | — | — | Описание сцены, действия, движения камеры и настроения видео. |
| `image` **(обязательный)** | string | — | — | URL стартового изображения, задающего генерацию. |
| `last_image` | string | — | — | URL изображения последнего кадра — для продолжения видео. |
| `aspect_ratio` | string | — | `16:9`, `9:16`, `4:3`, `3:4`, `1:1`, `21:9` | Соотношение сторон. Если не задано — подстраивается под входное изображение. |
| `resolution` | string | `720p` | `480p`, `720p`, `1080p`, `4k` | Разрешение выходного видео. |
| `duration` | integer | `5` | 4…15 | Длительность видео в секундах. |
| `enable_web_search` | boolean | `false` | — | Включить веб-поиск для актуальной информации. |
| `generate_audio` | boolean | `true` | — | Генерировать ли нативное аудио, синхронизированное с видео. **На стоимость не влияет.** |

### Пример тела запроса

```json
{
  "prompt": "Low-angle wide shot, athletic blonde woman with low side braid... crisp 4K 60fps, bright clean sport fashion aesthetic, upbeat energetic summer vibe, no text, no distortion.",
  "image": "https://static.wavespeed.ai/examples/.../1784106199966673991_PZr3eDmK-b417cff9e14e.webp",
  "aspect_ratio": "9:16",
  "resolution": "720p",
  "duration": 5,
  "enable_web_search": false,
  "generate_audio": true
}
```

---

## Выходные параметры

Поля объекта предсказания внутри `data`. `outputs` заполняется, когда `status` = `completed`.

| Параметр | Тип | Описание |
|---|---|---|
| `id` | string | Уникальный идентификатор предсказания (он же ID для запроса результата). |
| `model` | string | ID использованной модели. |
| `status` | string | Статус задачи: `created`, `processing`, `completed`, `failed`. |
| `outputs` | array&lt;string \| object&gt; | Массив результатов (пустой, пока статус не `completed`). Обычно URL-строки, но могут быть текстом или структурированными объектами — зависит от модели. |
| `urls` | object | Связанные API-эндпоинты (в т.ч. `urls.get`). |
| `created_at` | string | ISO-таймстамп создания запроса, например `2023-04-01T12:34:56.789Z`. |

---

## Полный пример (cURL / HTTP)

```bash
set -euo pipefail

export WAVESPEED_API_KEY="your-api-key"

REQUEST_BODY=$(cat <<'JSON'
{
  "prompt": "Low-angle wide shot, athletic blonde woman with low side braid, toned glistening abs covered in sweat, white sport sunglasses reflecting blue sky, thick white sweat wristbands on both wrists. She wears white racerback sports bra, unzipped lightweight black windbreaker, black high-waisted leggings. Dynamic slow subtle movement: right hand lifts to forehead to block harsh midday sun, left hand rests firmly on hip, head tilts slightly upward gazing far away. Bright scorching noon sunlight, hard high-contrast shadows, vivid saturated clear cerulean sky with wispy white clouds background. Vintage Kodak Gold 200 film texture, soft natural film grain, warm sharp highlights on glistening skin, natural skin texture, fast subtle camera slow pan right, crisp 4K 60fps, bright clean sport fashion aesthetic, upbeat energetic summer vibe, no text, no distortion.",
  "image": "https://static.wavespeed.ai/examples/3faf73bed9d84c3c9007d47e2d2578d0/1784106199966673991_PZr3eDmK-b417cff9e14e.webp",
  "aspect_ratio": "9:16",
  "resolution": "720p",
  "duration": 5,
  "enable_web_search": false,
  "generate_audio": true
}
JSON
)

# 1. Отправить задачу на генерацию.
SUBMIT_RESPONSE=$(curl --silent --show-error --fail-with-body \
  -X POST "https://api.wavespeed.ai/api/v3/bytedance/seedance-2.0/image-to-video" \
  -H "Authorization: Bearer ${WAVESPEED_API_KEY}" \
  -H "Content-Type: application/json" \
  -d "${REQUEST_BODY}")

TASK=$(printf '%s' "${SUBMIT_RESPONSE}" | jq 'if type == "object" and has("data") then .data else . end')
PREDICTION_ID=$(printf '%s' "${TASK}" | jq -r '.id // empty')
if [ -z "${PREDICTION_ID}" ]; then
  printf 'Submission response did not contain a prediction id\n' >&2
  exit 1
fi
RESULT_URL=$(printf '%s' "${TASK}" | jq -r '.urls.get // empty')
if [ -z "${RESULT_URL}" ]; then RESULT_URL="https://api.wavespeed.ai/api/v3/predictions/${PREDICTION_ID}/result"; fi

# 2. Опрашивать до завершения.
while true; do
  RESPONSE=$(curl --silent --show-error --fail-with-body \
    "${RESULT_URL}" \
    -H "Authorization: Bearer ${WAVESPEED_API_KEY}")
  RESULT=$(printf '%s' "${RESPONSE}" | jq 'if type == "object" and has("data") then .data else . end')
  STATUS=$(printf '%s' "${RESULT}" | jq -r '.status // empty')

  case "${STATUS}" in
    completed) printf '%s\n' "${RESULT}" | jq '.outputs'; break ;;
    failed|cancelled|timeout) printf '%s\n' "${RESULT}" | jq . >&2; exit 1 ;;
    created|processing) sleep 2 ;;
    *) printf 'Unexpected status: %s\n' "${STATUS}" >&2; exit 1 ;;
  esac
done
```

---

## Ответы API

### Ответ на отправку задачи

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": "abc123-def456",
    "model": "bytedance/seedance-2.0/image-to-video",
    "input": {
      "prompt": "Low-angle wide shot, athletic blonde woman...",
      "image": "https://static.wavespeed.ai/examples/.../1784106199966673991_PZr3eDmK-b417cff9e14e.webp",
      "aspect_ratio": "9:16",
      "resolution": "720p",
      "duration": 5,
      "enable_web_search": false,
      "generate_audio": true
    },
    "outputs": [],
    "status": "created",
    "created_at": "2026-01-01T00:00:00Z",
    "urls": {
      "get": "https://api.wavespeed.ai/api/v3/predictions/abc123-def456/result"
    },
    "code": 0,
    "error": "",
    "timings": {
      "inference": 0
    }
  }
}
```

### Ответ при завершении (polling)

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": "abc123-def456",
    "model": "bytedance/seedance-2.0/image-to-video",
    "input": { "...": "тот же input" },
    "outputs": [
      "https://cdn.wavespeed.ai/outputs/result.png"
    ],
    "status": "completed",
    "created_at": "2026-01-01T00:00:00Z",
    "urls": {
      "get": "https://api.wavespeed.ai/api/v3/predictions/abc123-def456/result"
    },
    "code": 0,
    "error": "",
    "timings": {
      "inference": 2500
    }
  }
}
```

---

## Заметки по интеграции

- Фолбэк URL результата, если `urls.get` не пришёл: `https://api.wavespeed.ai/api/v3/predictions/{id}/result`.
- Обрабатывать все терминальные статусы: `failed`, `cancelled`, `timeout` — не только `failed`.
- `generate_audio` не влияет на цену, отключать ради экономии смысла нет.
- Ссылки в `outputs` ведут на CDN — при необходимости долгого хранения скачивать и класть в своё хранилище.
