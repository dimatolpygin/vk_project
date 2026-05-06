#!/bin/bash
set -e

echo "=== VK Нейробот: установка сервера Ubuntu ==="

# Обновление системы
apt-get update && apt-get upgrade -y

# Установка базовых инструментов
apt-get install -y curl wget git make unzip

# Установка Docker
if ! command -v docker &> /dev/null; then
    echo "Устанавливаю Docker..."
    curl -fsSL https://get.docker.com | sh
    systemctl enable docker
    systemctl start docker
fi

# Установка Docker Compose Plugin
if ! docker compose version &> /dev/null; then
    echo "Устанавливаю Docker Compose..."
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

# Создание директории проекта
PROJECT_DIR="/opt/vk_neuro_bot"
mkdir -p "$PROJECT_DIR"
cd "$PROJECT_DIR"

# Клон репозитория (если ещё нет)
if [ ! -d ".git" ]; then
    echo "Введите URL репозитория:"
    read -r REPO_URL
    git clone "$REPO_URL" .
fi

# Создание .env из примера
if [ ! -f ".env" ]; then
    cp .env.example .env
    echo ""
    echo "=== ВАЖНО: отредактируйте .env перед запуском ==="
    echo "nano $PROJECT_DIR/.env"
    echo ""
fi

# Запуск инфраструктуры
docker compose up -d postgres redis

echo "Ожидание запуска PostgreSQL..."
sleep 5

# Применение миграций
source .env
goose -dir migrations postgres "$DB_URL" up

# Запуск всех сервисов
docker compose up -d

echo ""
echo "=== Установка завершена! ==="
echo "Бот:    http://$(curl -s ifconfig.me):8080"
echo "Админка: http://$(curl -s ifconfig.me):8081"
echo ""
echo "Для регистрации webhook запустите: ./scripts/set-webhooks.sh"
