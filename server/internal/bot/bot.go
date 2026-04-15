package bot

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"teledrive/server/internal/storage"
)

type Store interface {
	UpsertUserByTelegramID(ctx context.Context, telegramID int64) (storage.User, error)
	GetOrCreateActiveConfig(ctx context.Context, userID int64) (string, error)
	SetConfigMode(ctx context.Context, userID int64, mode string) error
	ListActiveSessions(ctx context.Context, userID int64) ([]storage.Session, error)
	DisconnectAllSessions(ctx context.Context, userID int64) error
	LogAction(ctx context.Context, userID int64, action string, payload string) error
}

type PaymentService interface {
	CreateSubscriptionPayment(ctx context.Context, userID int64, tariff string) (int64, float64, error)
	ConfirmPayment(ctx context.Context, paymentID int64) error
}

type WGBuilder interface {
	BuildClientConfig(userID int64, privateKey string) string
}

// Bot реализует Telegram-интерфейс для SaaS-кабинета.
type Bot struct {
	api            *tgbotapi.BotAPI
	store          Store
	paymentService PaymentService
	wgBuilder      WGBuilder
}

func New(token string, store Store, paymentService PaymentService, wgBuilder WGBuilder) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("init telegram bot: %w", err)
	}

	return &Bot{api: api, store: store, paymentService: paymentService, wgBuilder: wgBuilder}, nil
}

func (b *Bot) Run(ctx context.Context) error {
	cfg := tgbotapi.NewUpdate(0)
	cfg.Timeout = 30
	updates := b.api.GetUpdatesChan(cfg)

	for {
		select {
		case <-ctx.Done():
			return nil
		case u := <-updates:
			switch {
			case u.Message != nil:
				b.handleMessage(ctx, u.Message)
			case u.CallbackQuery != nil:
				b.handleCallback(ctx, u.CallbackQuery)
			}
		}
	}
}

func (b *Bot) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	user, err := b.store.UpsertUserByTelegramID(ctx, msg.From.ID)
	if err != nil {
		log.Printf("upsert user failed: %v", err)
		b.reply(msg.Chat.ID, "Временная ошибка. Попробуйте снова через минуту.")
		return
	}

	_ = b.store.LogAction(ctx, user.ID, "command", msg.Text)

	if !msg.IsCommand() {
		b.reply(msg.Chat.ID, "Используйте команды: /start /subscribe /config /devices /support /mode")
		return
	}

	switch msg.Command() {
	case "start":
		b.sendStart(msg.Chat.ID)
	case "subscribe":
		b.sendTariffKeyboard(msg.Chat.ID)
	case "config":
		b.sendConfig(ctx, msg.Chat.ID, user)
	case "devices":
		b.sendDevices(ctx, msg.Chat.ID, user)
	case "support":
		b.reply(msg.Chat.ID, "Поддержка: опишите проблему одним сообщением, оператор ответит в этом чате.")
	case "mode":
		b.sendModeKeyboard(msg.Chat.ID)
	default:
		b.reply(msg.Chat.ID, "Неизвестная команда. Используйте /start")
	}
}

func (b *Bot) handleCallback(ctx context.Context, q *tgbotapi.CallbackQuery) {
	user, err := b.store.UpsertUserByTelegramID(ctx, q.From.ID)
	if err != nil {
		log.Printf("upsert user on callback failed: %v", err)
		return
	}

	_ = b.store.LogAction(ctx, user.ID, "callback", q.Data)

	switch {
	case q.Data == "buy_basic":
		b.createPendingPayment(ctx, q.Message.Chat.ID, user.ID, "basic")
	case q.Data == "buy_standard":
		b.createPendingPayment(ctx, q.Message.Chat.ID, user.ID, "standard")
	case q.Data == "buy_pro":
		b.createPendingPayment(ctx, q.Message.Chat.ID, user.ID, "pro")
	case q.Data == "show_account":
		sub := "не активна"
		if user.SubscriptionEnd.Valid {
			sub = user.SubscriptionEnd.Time.Format(time.RFC3339)
		}
		b.reply(q.Message.Chat.ID, fmt.Sprintf("Тариф: %s\nЛимит устройств: %d\nПодписка до: %s", user.Tariff, user.MaxDevices, sub))
	case q.Data == "disconnect_all":
		if err := b.store.DisconnectAllSessions(ctx, user.ID); err != nil {
			log.Printf("disconnect all failed: %v", err)
			b.reply(q.Message.Chat.ID, "Не удалось отключить устройства. Повторите попытку позже.")
			return
		}
		b.reply(q.Message.Chat.ID, "Все активные сессии отключены.")
	case q.Data == "mode_full_tunnel":
		b.setMode(ctx, q.Message.Chat.ID, user.ID, "full_tunnel")
	case q.Data == "mode_split_tunnel":
		b.setMode(ctx, q.Message.Chat.ID, user.ID, "split_tunnel")
	case q.Data == "mode_per_app":
		b.setMode(ctx, q.Message.Chat.ID, user.ID, "per_app")
	case strings.HasPrefix(q.Data, "confirm_payment_"):
		idStr := strings.TrimPrefix(q.Data, "confirm_payment_")
		id, convErr := strconv.ParseInt(idStr, 10, 64)
		if convErr != nil {
			b.reply(q.Message.Chat.ID, "Некорректный идентификатор платежа")
			return
		}
		if err := b.paymentService.ConfirmPayment(ctx, id); err != nil {
			b.reply(q.Message.Chat.ID, "Не удалось подтвердить оплату")
			return
		}
		b.reply(q.Message.Chat.ID, "Оплата подтверждена, подписка активирована.")
	}

	callback := tgbotapi.NewCallback(q.ID, "Готово")
	if _, err := b.api.Request(callback); err != nil {
		log.Printf("callback ack failed: %v", err)
	}
}

func (b *Bot) sendStart(chatID int64) {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Купить подписку", "buy_basic"),
			tgbotapi.NewInlineKeyboardButtonData("Мой аккаунт", "show_account"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("Скачать клиент", "https://example.com/download"),
			tgbotapi.NewInlineKeyboardButtonURL("Инструкция", "https://example.com/help"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "Добро пожаловать в личный кабинет. Выберите действие ниже.")
	msg.ReplyMarkup = keyboard
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("send start failed: %v", err)
	}
}

func (b *Bot) sendTariffKeyboard(chatID int64) {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Basic — 299₽", "buy_basic"),
			tgbotapi.NewInlineKeyboardButtonData("Standard — 699₽", "buy_standard"),
			tgbotapi.NewInlineKeyboardButtonData("Pro — 1490₽", "buy_pro"),
		),
	)
	msg := tgbotapi.NewMessage(chatID, "Выберите тариф для оформления подписки.")
	msg.ReplyMarkup = keyboard
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("send tariff keyboard failed: %v", err)
	}
}

func (b *Bot) sendModeKeyboard(chatID int64) {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Full tunnel", "mode_full_tunnel"),
			tgbotapi.NewInlineKeyboardButtonData("Split tunnel", "mode_split_tunnel"),
			tgbotapi.NewInlineKeyboardButtonData("Per app", "mode_per_app"),
		),
	)
	msg := tgbotapi.NewMessage(chatID, "Выберите режим маршрутизации.")
	msg.ReplyMarkup = keyboard
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("send mode keyboard failed: %v", err)
	}
}

func (b *Bot) setMode(ctx context.Context, chatID int64, userID int64, mode string) {
	if err := b.store.SetConfigMode(ctx, userID, mode); err != nil {
		log.Printf("set mode failed: %v", err)
		b.reply(chatID, "Не удалось изменить режим. Попробуйте позже.")
		return
	}
	b.reply(chatID, "Режим обновлён: "+mode)
}

func (b *Bot) createPendingPayment(ctx context.Context, chatID int64, userID int64, tariff string) {
	if b.paymentService == nil {
		b.reply(chatID, "Платежный сервис не настроен.")
		return
	}
	paymentID, amount, err := b.paymentService.CreateSubscriptionPayment(ctx, userID, tariff)
	if err != nil {
		log.Printf("create payment failed: %v", err)
		b.reply(chatID, "Не удалось создать платеж. Попробуйте позже.")
		return
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Платеж #%d создан на %.0f₽. Нажмите кнопку ниже для demo-подтверждения.", paymentID, amount))
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Подтвердить оплату (demo)", fmt.Sprintf("confirm_payment_%d", paymentID)),
		),
	)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("send payment message failed: %v", err)
	}
}

func (b *Bot) sendConfig(ctx context.Context, chatID int64, user storage.User) {
	key, err := b.store.GetOrCreateActiveConfig(ctx, user.ID)
	if err != nil {
		log.Printf("get config failed: %v", err)
		b.reply(chatID, "Не удалось получить конфиг.")
		return
	}

	if b.wgBuilder == nil {
		b.reply(chatID, "WG builder не настроен на сервере.")
		return
	}

	cfgText := b.wgBuilder.BuildClientConfig(user.ID, key)
	b.reply(chatID, "Ваш WireGuard-конфиг:\n\n"+cfgText)
}

func (b *Bot) sendDevices(ctx context.Context, chatID int64, user storage.User) {
	sessions, err := b.store.ListActiveSessions(ctx, user.ID)
	if err != nil {
		log.Printf("list sessions failed: %v", err)
		b.reply(chatID, "Не удалось получить список устройств.")
		return
	}

	if len(sessions) == 0 {
		b.reply(chatID, "Активных устройств нет.")
		return
	}

	lines := make([]string, 0, len(sessions)+1)
	lines = append(lines, "Активные устройства:")
	for _, s := range sessions {
		ip := "n/a"
		if s.IP.Valid {
			ip = s.IP.String
		}
		lines = append(lines, fmt.Sprintf("• %s | %s | ip: %s", s.DeviceID, s.ConnectedAt.Format(time.RFC3339), ip))
	}
	msg := tgbotapi.NewMessage(chatID, strings.Join(lines, "\n"))
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Отключить всё", "disconnect_all")),
	)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("send sessions failed: %v", err)
	}
}

func (b *Bot) reply(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("send telegram message failed: %v", err)
	}
}
