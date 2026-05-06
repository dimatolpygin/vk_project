#!/bin/bash
set -e

echo "=== VK Нейробот: установка сервера Ubuntu ==="

# Обновление системы
apt-get update && apt-get upgrade -y

# Установка зависимостей
apt-get install -y curl wget git make unzip nginx certbot python3-certbot-nginx

# Установка Docker
if ! command -v docker &> /dev/null; then
    echo "Устанавливаю Docker..."
    curl -fsSL https://get.docker.com | sh
    systemctl enable docker
    systemctl start docker
fi

# Установка Docker Compose Plugin
if ! docker compose version &> /dev/null; then
    echo "Устанавливаю Docker Compose Plugin..."
    apt-get install -y docker-compose-plugin
fi

# Установка goose
if ! command -v goose &> /dev/null; then
    echo "Устанавливаю goose..."
    GOOSE_VERSION="v3.20.0"
    curl -fsSL "https://github.com/pressly/goose/releases/download/${GOOSE_VERSION}/goose_linux_x86_64" \
        -o /usr/local/bin/goose
    chmod +x /usr/local/bin/goose
fi

# ── Директория проекта ───────────────────────────────────────────────────────
PROJECT_DIR="/opt/vk_neuro_bot"
mkdir -p "$PROJECT_DIR"
cd "$PROJECT_DIR"

if [ ! -d ".git" ]; then
    echo "Введите URL репозитория (например https://github.com/user/repo):"
    read -r REPO_URL
    git clone "$REPO_URL" .
fi

# Создание .env из примера (если ещё нет)
if [ ! -f ".env" ]; then
    cp .env.example .env
    echo ""
    echo "=== ВАЖНО: заполните .env, затем запустите скрипт снова ==="
    echo ""
    echo "  nano $PROJECT_DIR/.env"
    echo ""
    echo "Обязательные поля:"
    echo "  BOT_WEBHOOK_URL=https://yourdomain.com"
    echo "  CERTBOT_EMAIL=your@email.com"
    echo "  VK_GROUP_TOKEN, VK_GROUP_ID, VK_CONFIRMATION_TOKEN, VK_SECRET"
    echo "  YUKASSA_SHOP_ID, YUKASSA_SECRET_KEY, YUKASSA_WEBHOOK_SECRET"
    echo "  WAVESPEED_API_KEY, WAVESPEED_WEBHOOK_SECRET"
    echo "  S3_ENDPOINT, S3_BUCKET, S3_ACCESS_KEY, S3_SECRET_KEY"
    echo ""
    exit 0
fi

# Загрузка переменных
set -a
source .env
set +a

# ── Валидация ключевых переменных ────────────────────────────────────────────
DOMAIN=$(echo "$BOT_WEBHOOK_URL" | sed 's|https\?://||' | cut -d'/' -f1)

if [ -z "$DOMAIN" ]; then
    echo "ОШИБКА: BOT_WEBHOOK_URL не задан. Пример: BOT_WEBHOOK_URL=https://example.com"
    exit 1
fi
if [ -z "$CERTBOT_EMAIL" ]; then
    echo "ОШИБКА: CERTBOT_EMAIL не задан в .env"
    exit 1
fi

echo ""
echo "Домен: $DOMAIN"
echo "Email: $CERTBOT_EMAIL"
echo ""

# ── nginx: конфиг до выдачи сертификата (HTTP) ──────────────────────────────
echo "Настраиваю nginx (HTTP)..."

cat > /etc/nginx/sites-available/vkbot << NGINX_CONF
server {
    listen 80;
    server_name ${DOMAIN};

    # Бот (VK webhook, ЮKassa callback, WaveSpeed webhook)
    location / {
        proxy_pass         http://127.0.0.1:${BOT_PORT:-8080};
        proxy_http_version 1.1;
        proxy_set_header   Host              \$host;
        proxy_set_header   X-Real-IP         \$remote_addr;
        proxy_set_header   X-Forwarded-For   \$proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto \$scheme;
        proxy_read_timeout 30s;
    }

    # Админ-панель — доступна только с разрешённых IP
    # (замените 0.0.0.0/0 на ваш IP для безопасности)
    location /admin {
        allow  0.0.0.0/0;
        deny   all;
        proxy_pass         http://127.0.0.1:${ADMIN_PORT:-8081};
        proxy_http_version 1.1;
        proxy_set_header   Host              \$host;
        proxy_set_header   X-Real-IP         \$remote_addr;
        proxy_set_header   X-Forwarded-For   \$proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto \$scheme;
    }
}
NGINX_CONF

ln -sf /etc/nginx/sites-available/vkbot /etc/nginx/sites-enabled/vkbot
rm -f /etc/nginx/sites-enabled/default

nginx -t
systemctl enable nginx
systemctl restart nginx

# ── Docker: запуск инфраструктуры ────────────────────────────────────────────
echo "Запускаю postgres и redis..."
docker compose up -d postgres redis

echo "Ожидание запуска PostgreSQL (10 сек)..."
sleep 10

echo "Применяю миграции..."
goose -dir migrations postgres "$DB_URL" up

echo "Запускаю бот и воркер..."
docker compose up -d

echo "Ожидание запуска бота (5 сек)..."
sleep 5

# ── Certbot: выдача SSL-сертификата ─────────────────────────────────────────
echo "Получаю SSL-сертификат для ${DOMAIN}..."

# --nginx   → certbot сам модифицирует конфиг и добавляет SSL
# --redirect → добавляет редирект HTTP → HTTPS
certbot --nginx \
    -d "${DOMAIN}" \
    --non-interactive \
    --agree-tos \
    --email "${CERTBOT_EMAIL}" \
    --redirect

systemctl reload nginx

# ── Автообновление сертификата ───────────────────────────────────────────────
cat > /etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh << 'HOOK'
#!/bin/bash
systemctl reload nginx
HOOK
chmod +x /etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh

# systemd timer для автообновления (certbot пакет уже включает его)
systemctl enable certbot.timer 2>/dev/null || true

echo ""
echo "================================================================"
echo "  Установка завершена!"
echo "================================================================"
echo ""
echo "  Бот (webhook):   https://${DOMAIN}/"
echo "  Админ-панель:    https://${DOMAIN}/admin"
echo ""
echo "  Следующий шаг — зарегистрировать webhook в VK:"
echo "    cd $PROJECT_DIR && ./scripts/set-webhooks.sh"
echo ""
echo "  Статус контейнеров:"
docker compose ps
echo ""
