package config

import (
	"fmt"
	"os"
)

// Config хранит runtime-конфигурацию сервиса.
type Config struct {
	Env               string
	HTTPAddr          string
	TelegramToken     string
	DatabaseURL       string
	RedisAddr         string
	WGEndpoint        string
	WGServerPublicKey string
	WGDNS             string
	AdminToken        string
}

func Load() (Config, error) {
	cfg := Config{
		Env:               envOrDefault("APP_ENV", "development"),
		HTTPAddr:          envOrDefault("HTTP_ADDR", ":8080"),
		TelegramToken:     os.Getenv("TELEGRAM_BOT_TOKEN"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		RedisAddr:         envOrDefault("REDIS_ADDR", "localhost:6379"),
		WGEndpoint:        envOrDefault("WG_ENDPOINT", "vpn.example.com:51820"),
		WGServerPublicKey: envOrDefault("WG_SERVER_PUBLIC_KEY", "server-public-key-placeholder"),
		WGDNS:             envOrDefault("WG_DNS", "1.1.1.1"),
		AdminToken:        os.Getenv("ADMIN_TOKEN"),
	}

	if cfg.TelegramToken == "" {
		return Config{}, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.RedisAddr == "" {
		return Config{}, fmt.Errorf("REDIS_ADDR is required")
	}

	return cfg, nil
}

func envOrDefault(name, fallback string) string {
	v := os.Getenv(name)
	if v == "" {
		return fallback
	}
	return v
}
