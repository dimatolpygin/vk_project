.PHONY: dev build test lint migrate migrate-down migrate-status tidy docker-up docker-down

# Локальная разработка с hot-reload через air
dev:
	air -c .air.toml

# Сборка бинарников
build:
	go build -o bin/bot ./cmd/bot
	go build -o bin/worker ./cmd/worker

# Тесты
test:
	go test ./... -v

# Линтер
lint:
	golangci-lint run ./...

# Применить миграции (goose up)
migrate:
	goose -dir migrations postgres "$(shell grep DB_URL .env | cut -d= -f2-)" up

# Откатить последнюю миграцию
migrate-down:
	goose -dir migrations postgres "$(shell grep DB_URL .env | cut -d= -f2-)" down

# Статус миграций
migrate-status:
	goose -dir migrations postgres "$(shell grep DB_URL .env | cut -d= -f2-)" status

# Подтянуть зависимости
tidy:
	go mod tidy

# Docker: запустить все сервисы (продакшн)
docker-up:
	docker compose up -d

# Docker: остановить все сервисы
docker-down:
	docker compose down

# Docker: запустить только инфраструктуру (postgres + redis) для локальной разработки
docker-infra:
	docker compose -f docker-compose.dev.yml up -d postgres redis

# Логи сервисов
logs:
	docker compose logs -f

# Пересобрать и перезапустить
deploy:
	docker compose build
	docker compose up -d
