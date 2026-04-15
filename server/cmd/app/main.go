package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"teledrive/server/internal/admin"
	"teledrive/server/internal/bot"
	"teledrive/server/internal/config"
	httpapi "teledrive/server/internal/http"
	"teledrive/server/internal/limiter"
	"teledrive/server/internal/payments"
	"teledrive/server/internal/storage"
	"teledrive/server/internal/subscription"
	"teledrive/server/internal/wg"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := storage.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("init store: %v", err)
	}
	defer func() { _ = store.Close() }()
	if err := store.Ping(ctx); err != nil {
		log.Fatalf("db ping failed: %v", err)
	}

	deviceLimiter := limiter.New(cfg.RedisAddr)
	defer func() { _ = deviceLimiter.Close() }()
	if err := deviceLimiter.Ping(ctx); err != nil {
		log.Fatalf("redis ping failed: %v", err)
	}

	wgBuilder := wg.NewBuilder(cfg.WGEndpoint, cfg.WGServerPublicKey, cfg.WGDNS)
	paymentService := payments.NewService(store)
	subscriptionJob := subscription.NewJob(store)

	httpServer := httpapi.New(cfg.HTTPAddr, store, deviceLimiter, paymentService, wgBuilder)
	adminHandler, err := admin.NewHandler(store, cfg.AdminToken)
	if err != nil {
		log.Fatalf("init admin handler: %v", err)
	}
	httpServer.RegisterModule(adminHandler)

	tgBot, err := bot.New(cfg.TelegramToken, store, paymentService, wgBuilder)
	if err != nil {
		log.Fatalf("init bot: %v", err)
	}

	go subscriptionJob.Run(ctx)

	go func() {
		if err := httpServer.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server failed: %v", err)
		}
	}()

	go func() {
		if err := tgBot.Run(ctx); err != nil {
			log.Fatalf("bot failed: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}
