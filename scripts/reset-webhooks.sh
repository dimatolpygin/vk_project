#!/bin/bash
set -e

if [ -f .env ]; then
    export $(grep -v '^#' .env | xargs)
fi

if [ -z "$VK_GROUP_TOKEN" ] || [ -z "$VK_GROUP_ID" ]; then
    echo "ОШИБКА: не заданы VK_GROUP_TOKEN или VK_GROUP_ID в .env"
    exit 1
fi

echo "=== Сброс всех Callback-серверов VK ==="
echo "Group ID: $VK_GROUP_ID"
echo ""

echo "1. Получаю список зарегистрированных серверов..."
SERVERS=$(curl -s "https://api.vk.com/method/groups.getCallbackServers" \
    -d "group_id=${VK_GROUP_ID}" \
    -d "access_token=${VK_GROUP_TOKEN}" \
    -d "v=5.131")

echo "$SERVERS"
echo ""

SERVER_IDS=$(echo "$SERVERS" | python3 -c "
import sys, json
data = json.load(sys.stdin)
items = data.get('response', {}).get('items', [])
for item in items:
    print(item['id'])
" 2>/dev/null || echo "")

if [ -z "$SERVER_IDS" ]; then
    echo "Серверов не найдено или ошибка получения списка."
else
    echo "2. Удаляю серверы..."
    while IFS= read -r sid; do
        if [ -n "$sid" ]; then
            echo "   Удаляю server_id=$sid ..."
            curl -s "https://api.vk.com/method/groups.deleteCallbackServer" \
                -d "group_id=${VK_GROUP_ID}" \
                -d "server_id=${sid}" \
                -d "access_token=${VK_GROUP_TOKEN}" \
                -d "v=5.131"
            echo ""
        fi
    done <<< "$SERVER_IDS"
fi

echo ""
echo "=== Все серверы удалены. Регистрирую новый... ==="
echo ""

bash "$(dirname "$0")/set-webhooks.sh"
