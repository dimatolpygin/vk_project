package config

import (
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

type Config struct {
	// VK
	VKGroupToken        string
	VKGroupID           int64
	VKSecret            string
	VKConfirmationToken string

	// WaveSpeed
	WavespeedAPIKey        string
	WavespeedModel         string
	WavespeedWebhookSecret string

	// YuKassa
	YukassaShopID                string
	YukassaSecretKey             string
	YukassaWebhookSecret         string
	YukassaReceiptEmail          string
	YukassaReceiptVATCode        int
	YukassaReceiptPaymentSubject string
	YukassaReceiptPaymentMode    string

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
	PublicBaseURL string
	BotWebhookURL string
	LogLevel      string
}

func Load() *Config {
	_ = godotenv.Load()

	botWebhookURL := getEnv("BOT_WEBHOOK_URL", "")

	cfg := &Config{
		VKGroupToken:        mustEnv("VK_GROUP_TOKEN"),
		VKGroupID:           parseInt64(getEnv("VK_GROUP_ID", "0")),
		VKSecret:            getEnv("VK_SECRET", ""),
		VKConfirmationToken: mustEnv("VK_CONFIRMATION_TOKEN"),

		WavespeedAPIKey:        mustEnv("WAVESPEED_API_KEY"),
		WavespeedModel:         getEnv("WAVESPEED_MODEL", "google/nano-banana-pro"),
		WavespeedWebhookSecret: getEnv("WAVESPEED_WEBHOOK_SECRET", ""),

		YukassaShopID:                mustEnv("YUKASSA_SHOP_ID"),
		YukassaSecretKey:             mustEnv("YUKASSA_SECRET_KEY"),
		YukassaWebhookSecret:         getEnv("YUKASSA_WEBHOOK_SECRET", ""),
		YukassaReceiptEmail:          getEnv("YUKASSA_RECEIPT_EMAIL", ""),
		YukassaReceiptVATCode:        parseInt(getEnv("YUKASSA_RECEIPT_VAT_CODE", "1"), 1),
		YukassaReceiptPaymentSubject: getEnv("YUKASSA_RECEIPT_PAYMENT_SUBJECT", "service"),
		YukassaReceiptPaymentMode:    getEnv("YUKASSA_RECEIPT_PAYMENT_MODE", "full_prepayment"),

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

		PublicBaseURL: derivePublicBaseURL(getEnv("PUBLIC_BASE_URL", ""), botWebhookURL),
		BotWebhookURL: botWebhookURL,
		LogLevel:      getEnv("LOG_LEVEL", "info"),
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

func parseInt(s string, fallback int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}

func derivePublicBaseURL(publicBaseURL, botWebhookURL string) string {
	if normalized := normalizeBaseURL(publicBaseURL); normalized != "" {
		return normalized
	}
	return normalizeBaseURL(botWebhookURL)
}

func normalizeBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.TrimRight(raw, "/")
	}

	return strings.TrimRight(parsed.Scheme+"://"+parsed.Host, "/")
}
