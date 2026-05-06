@echo off
chcp 65001 > nul

echo [VK Нейробот] Запуск локальной среды...

REM Проверяем наличие .env
if not exist .env (
    echo ОШИБКА: файл .env не найден. Скопируйте .env.example в .env и заполните.
    pause
    exit /b 1
)

REM Запускаем docker-инфраструктуру
echo [1/3] Запуск PostgreSQL и Redis...
docker compose -f docker-compose.dev.yml up -d postgres redis
if errorlevel 1 (
    echo ОШИБКА: не удалось запустить Docker. Убедитесь, что Docker Desktop запущен.
    pause
    exit /b 1
)

REM Ждём готовности postgres
echo [2/3] Ожидание готовности PostgreSQL...
timeout /t 3 /nobreak > nul

REM Запускаем бота с air (hot-reload)
echo [3/3] Запуск бота...
if exist "%LOCALAPPDATA%\go\bin\air.exe" (
    "%LOCALAPPDATA%\go\bin\air.exe" -c .air.toml
) else (
    go run ./cmd/bot
)

pause
