package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

type Config struct {
	// VK
	VKGroupToken       string
	VKGroupID          int64
	VKSecret           string
	VKConfirmationToken string

	// WaveSpeed
	WavespeedAPIKey       string
	WavespeedModel        string
	WavespeedWebhookSecret string

	// YuKassa
	YukassaShopID         string
	YukassaSecretKey      string
	YukassaWebhookSecret  string

	// DB
	DBURL string

	// Redis
	RedisAddr     string
	RedisPassword string

	// S3
	S3Endpoint  string
	S3Bucket    string
	S3AccessKey string
	S3SecretKey string
	S3Region    string

	// Admin
	AdminLogin    string
	AdminPassword string
	AdminPort     string
	BotPort       string

	// General
	BotWebhookURL string
	LogLevel      string
	VKProxy       string
}

func Load() *Config {
	_ = godotenv.Load()

	cfg := &Config{
		VKGroupToken:        mustEnv("VK_GROUP_TOKEN"),
		VKGroupID:           parseInt64(getEnv("VK_GROUP_ID", "0")),
		VKSecret:            getEnv("VK_SECRET", ""),
		VKConfirmationToken: mustEnv("VK_CONFIRMATION_TOKEN"),

		WavespeedAPIKey:        mustEnv("WAVESPEED_API_KEY"),
		WavespeedModel:         getEnv("WAVESPEED_MODEL", "nano-banana-pro"),
		WavespeedWebhookSecret: getEnv("WAVESPEED_WEBHOOK_SECRET", ""),

		YukassaShopID:        mustEnv("YUKASSA_SHOP_ID"),
		YukassaSecretKey:     mustEnv("YUKASSA_SECRET_KEY"),
		YukassaWebhookSecret: getEnv("YUKASSA_WEBHOOK_SECRET", ""),

		DBURL: mustEnv("DB_URL"),

		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		S3Endpoint:  getEnv("S3_ENDPOINT", ""),
		S3Bucket:    getEnv("S3_BUCKET", ""),
		S3AccessKey: getEnv("S3_ACCESS_KEY", ""),
		S3SecretKey: getEnv("S3_SECRET_KEY", ""),
		S3Region:    getEnv("S3_REGION", "ru-1"),

		AdminLogin:    getEnv("ADMIN_LOGIN", "admin"),
		AdminPassword: getEnv("ADMIN_PASSWORD", "changeme"),
		AdminPort:     getEnv("ADMIN_PORT", "8081"),
		BotPort:       getEnv("BOT_PORT", "8080"),

		BotWebhookURL: getEnv("BOT_WEBHOOK_URL", ""),
		LogLevel:      getEnv("LOG_LEVEL", "info"),
		VKProxy:       getEnv("VK_PROXY", ""),
	}
	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatal().Str("key", key).Msg("обязательная переменная окружения не задана")
	}
	return v
}

func parseInt64(s string) int64 {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}
