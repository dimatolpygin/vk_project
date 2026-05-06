#!/bin/bash
set -e

# Загружаем .env
if [ -f .env ]; then
    export $(grep -v '^#' .env | xargs)
fi

echo "=== Регистрация VK Callback API webhook ==="

if [ -z "$VK_GROUP_TOKEN" ] || [ -z "$VK_GROUP_ID" ] || [ -z "$BOT_WEBHOOK_URL" ]; then
    echo "ОШИБКА: не заданы VK_GROUP_TOKEN, VK_GROUP_ID или BOT_WEBHOOK_URL в .env"
    exit 1
fi

WEBHOOK_URL="${BOT_WEBHOOK_URL}/vk/webhook"

echo "URL вебхука: $WEBHOOK_URL"
echo "Group ID:    $VK_GROUP_ID"

# Добавляем Callback-сервер
echo ""
echo "1. Добавляю Callback-сервер в VK..."
RESPONSE=$(curl -s "https://api.vk.com/method/groups.addCallbackServer" \
    -d "group_id=${VK_GROUP_ID}" \
    -d "url=${WEBHOOK_URL}" \
    -d "title=vk_neuro_bot" \
    -d "secret_key=${VK_SECRET}" \
    -d "access_token=${VK_GROUP_TOKEN}" \
    -d "v=5.131")

echo "$RESPONSE"

SERVER_ID=$(echo "$RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('response',{}).get('server_id',''))" 2>/dev/null || echo "")

if [ -z "$SERVER_ID" ]; then
    echo "Не удалось получить server_id. Проверьте параметры и попробуйте снова."
    exit 1
fi

echo "Server ID: $SERVER_ID"

# Настраиваем события Callback API
echo ""
echo "2. Настраиваю обрабатываемые события..."
curl -s "https://api.vk.com/method/groups.setCallbackSettings" \
    -d "group_id=${VK_GROUP_ID}" \
    -d "server_id=${SERVER_ID}" \
    -d "message_new=1" \
    -d "message_event=1" \
    -d "access_token=${VK_GROUP_TOKEN}" \
    -d "v=5.131"

echo ""
echo ""
echo "=== Webhook зарегистрирован! ==="
echo ""
echo "Следующий шаг: убедитесь, что бот доступен по адресу ${WEBHOOK_URL}"
echo "и введите код подтверждения (VK_CONFIRMATION_TOKEN) из настроек группы VK."
