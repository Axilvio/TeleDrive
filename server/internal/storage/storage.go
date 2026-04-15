package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"teledrive/server/internal/wg"
)

type Store struct {
	db *sql.DB
}

type User struct {
	ID              int64
	TelegramID      int64
	Tariff          string
	MaxDevices      int
	SubscriptionEnd sql.NullTime
}

type Session struct {
	DeviceID    string
	ConnectedAt time.Time
	IP          sql.NullString
}

type Reminder struct {
	UserID     int64
	TelegramID int64
	Type       string
	Message    string
}

func New(databaseURL string) (*Store, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)

	return &Store{db: db}, nil
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) UpsertUserByTelegramID(ctx context.Context, telegramID int64) (User, error) {
	query := `
		INSERT INTO users (telegram_id, tariff, max_devices)
		VALUES ($1, 'basic', 1)
		ON CONFLICT (telegram_id) DO UPDATE
		SET updated_at = NOW()
		RETURNING id, telegram_id, tariff, max_devices, subscription_end
	`

	var u User
	if err := s.db.QueryRowContext(ctx, query, telegramID).Scan(
		&u.ID,
		&u.TelegramID,
		&u.Tariff,
		&u.MaxDevices,
		&u.SubscriptionEnd,
	); err != nil {
		return User{}, fmt.Errorf("upsert user: %w", err)
	}

	return u, nil
}

func (s *Store) CreatePendingPayment(ctx context.Context, userID int64, amount float64, provider, tariff string) (int64, error) {
	meta, _ := json.Marshal(map[string]string{"tariff": tariff})
	var paymentID int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO payments (user_id, amount, status, provider, metadata)
		VALUES ($1, $2, 'pending', $3, $4::jsonb)
		RETURNING id
	`, userID, amount, provider, string(meta)).Scan(&paymentID)
	if err != nil {
		return 0, fmt.Errorf("create payment: %w", err)
	}
	return paymentID, nil
}

func (s *Store) ConfirmPaymentAndActivate(ctx context.Context, paymentID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var userID int64
	var status string
	var metadata []byte
	if err := tx.QueryRowContext(ctx, `
		SELECT user_id, status, metadata
		FROM payments
		WHERE id = $1
		FOR UPDATE
	`, paymentID).Scan(&userID, &status, &metadata); err != nil {
		return fmt.Errorf("select payment: %w", err)
	}
	if status == "paid" {
		return tx.Commit()
	}

	var meta struct {
		Tariff string `json:"tariff"`
	}
	_ = json.Unmarshal(metadata, &meta)
	if meta.Tariff == "" {
		meta.Tariff = "basic"
	}

	maxDevices := tariffToMaxDevices(meta.Tariff)
	if _, err := tx.ExecContext(ctx, `
		UPDATE payments
		SET status = 'paid', paid_at = NOW()
		WHERE id = $1
	`, paymentID); err != nil {
		return fmt.Errorf("update payment: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET tariff = $2,
			max_devices = $3,
			subscription_end = GREATEST(COALESCE(subscription_end, NOW()), NOW()) + INTERVAL '30 days',
			updated_at = NOW()
		WHERE id = $1
	`, userID, meta.Tariff, maxDevices); err != nil {
		return fmt.Errorf("activate subscription: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO action_logs (user_id, action, payload)
		VALUES ($1, 'payment_confirmed', $2)
	`, userID, fmt.Sprintf("payment_id=%d,tariff=%s", paymentID, meta.Tariff)); err != nil {
		return fmt.Errorf("log payment confirmation: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (s *Store) GetOrCreateActiveConfig(ctx context.Context, userID int64) (string, error) {
	var cfg string
	err := s.db.QueryRowContext(ctx, `
		SELECT wg_private_key
		FROM configs
		WHERE user_id = $1 AND active = TRUE
		ORDER BY id DESC
		LIMIT 1
	`, userID).Scan(&cfg)
	if err == nil {
		return cfg, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("get config: %w", err)
	}

	privateKey, genErr := wg.GeneratePrivateKey()
	if genErr != nil {
		return "", fmt.Errorf("generate wg key: %w", genErr)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO configs (user_id, wg_private_key, vk_link, current_creds_json, active, tunnel_mode)
		VALUES ($1, $2, '', '{}'::jsonb, TRUE, 'full_tunnel')
	`, userID, privateKey)
	if err != nil {
		return "", fmt.Errorf("insert config: %w", err)
	}

	return privateKey, nil
}

func (s *Store) ListActiveSessions(ctx context.Context, userID int64) ([]Session, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT device_id, connected_at, COALESCE(host(ip), '')
		FROM sessions
		WHERE user_id = $1 AND disconnected_at IS NULL
		ORDER BY connected_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	result := make([]Session, 0)
	for rows.Next() {
		var sss Session
		if err := rows.Scan(&sss.DeviceID, &sss.ConnectedAt, &sss.IP); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		result = append(result, sss)
	}

	return result, rows.Err()
}

func (s *Store) CreateOrRefreshSession(ctx context.Context, userID int64, deviceID, ip string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (user_id, device_id, ip, connected_at, disconnected_at)
		VALUES ($1, $2, NULLIF($3, '')::inet, NOW(), NULL)
		ON CONFLICT (user_id, device_id) WHERE disconnected_at IS NULL
		DO UPDATE SET
			connected_at = NOW(),
			ip = EXCLUDED.ip,
			disconnected_at = NULL
	`, userID, deviceID, ip)
	if err != nil {
		return fmt.Errorf("upsert session: %w", err)
	}
	return nil
}

func (s *Store) DisconnectSession(ctx context.Context, userID int64, deviceID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE sessions
		SET disconnected_at = NOW()
		WHERE user_id = $1 AND device_id = $2 AND disconnected_at IS NULL
	`, userID, deviceID)
	if err != nil {
		return fmt.Errorf("disconnect session: %w", err)
	}
	return nil
}

func (s *Store) CountActiveSessions(ctx context.Context, userID int64) (int, error) {
	var cnt int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM sessions
		WHERE user_id = $1 AND disconnected_at IS NULL
	`, userID).Scan(&cnt); err != nil {
		return 0, fmt.Errorf("count sessions: %w", err)
	}
	return cnt, nil
}

func (s *Store) SetConfigMode(ctx context.Context, userID int64, mode string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE configs
		SET tunnel_mode = $2, updated_at = NOW()
		WHERE user_id = $1 AND active = TRUE
	`, userID, mode)
	if err != nil {
		return fmt.Errorf("set config mode: %w", err)
	}
	return nil
}

func (s *Store) DisconnectAllSessions(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE sessions
		SET disconnected_at = NOW()
		WHERE user_id = $1 AND disconnected_at IS NULL
	`, userID)
	if err != nil {
		return fmt.Errorf("disconnect sessions: %w", err)
	}
	return nil
}

func (s *Store) QueueSubscriptionReminders(ctx context.Context, daysBefore int) (int64, error) {
	kind := fmt.Sprintf("subscription_reminder_%dd", daysBefore)
	query := `
		INSERT INTO notifications_queue (user_id, kind, payload, scheduled_for)
		SELECT u.id,
		       $1,
		       jsonb_build_object('days_before', $2, 'telegram_id', u.telegram_id),
		       NOW()
		FROM users u
		WHERE u.subscription_end IS NOT NULL
		  AND DATE(u.subscription_end) = CURRENT_DATE + ($2::text || ' days')::interval
		  AND NOT EXISTS (
		      SELECT 1
		      FROM notifications_queue nq
		      WHERE nq.user_id = u.id
		        AND nq.kind = $1
		        AND DATE(nq.created_at) = CURRENT_DATE
		  )
	`
	res, err := s.db.ExecContext(ctx, query, kind, daysBefore)
	if err != nil {
		return 0, fmt.Errorf("queue reminders: %w", err)
	}
	rows, _ := res.RowsAffected()
	return rows, nil
}

func (s *Store) DisableExpiredUsers(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE configs c
		SET active = FALSE, updated_at = NOW()
		FROM users u
		WHERE c.user_id = u.id
		  AND u.subscription_end IS NOT NULL
		  AND u.subscription_end < NOW()
		  AND c.active = TRUE
	`)
	if err != nil {
		return 0, fmt.Errorf("disable expired configs: %w", err)
	}
	rows, _ := res.RowsAffected()
	return rows, nil
}

func (s *Store) LogAction(ctx context.Context, userID int64, action string, payload string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO action_logs (user_id, action, payload)
		VALUES ($1, $2, $3)
	`, userID, action, payload)
	if err != nil {
		return fmt.Errorf("log action: %w", err)
	}
	return nil
}

type AdminStats struct {
	UsersTotal      int
	ActiveConfigs   int
	ActiveSessions  int
	PendingPayments int
	PendingNotifyQ  int
}

type UserRow struct {
	ID              int64
	TelegramID      int64
	Tariff          string
	MaxDevices      int
	SubscriptionEnd string
}

func (s *Store) AdminStats(ctx context.Context) (AdminStats, error) {
	stats := AdminStats{}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&stats.UsersTotal); err != nil {
		return AdminStats{}, fmt.Errorf("count users: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM configs WHERE active = TRUE`).Scan(&stats.ActiveConfigs); err != nil {
		return AdminStats{}, fmt.Errorf("count active configs: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE disconnected_at IS NULL`).Scan(&stats.ActiveSessions); err != nil {
		return AdminStats{}, fmt.Errorf("count active sessions: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM payments WHERE status = 'pending'`).Scan(&stats.PendingPayments); err != nil {
		return AdminStats{}, fmt.Errorf("count pending payments: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notifications_queue WHERE status = 'pending'`).Scan(&stats.PendingNotifyQ); err != nil {
		return AdminStats{}, fmt.Errorf("count pending notifications: %w", err)
	}
	return stats, nil
}

func (s *Store) ListUsers(ctx context.Context, limit int) ([]UserRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, telegram_id, tariff, max_devices, COALESCE(TO_CHAR(subscription_end, 'YYYY-MM-DD HH24:MI:SS'), '-')
		FROM users
		ORDER BY id DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := make([]UserRow, 0, limit)
	for rows.Next() {
		var u UserRow
		if err := rows.Scan(&u.ID, &u.TelegramID, &u.Tariff, &u.MaxDevices, &u.SubscriptionEnd); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

type ServerInstanceRow struct {
	ID     int64
	Name   string
	Region string
	Host   string
	Port   int
	Active bool
}

type PaymentRow struct {
	ID        int64
	UserID    int64
	Amount    string
	Status    string
	Provider  string
	CreatedAt string
}

type NotificationRow struct {
	ID           int64
	UserID       int64
	Kind         string
	Status       string
	ScheduledFor string
}

func (s *Store) ListServerInstances(ctx context.Context, limit int) ([]ServerInstanceRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, COALESCE(region, ''), host, port, active
		FROM server_instances
		ORDER BY id DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list server instances: %w", err)
	}
	defer rows.Close()
	out := make([]ServerInstanceRow, 0, limit)
	for rows.Next() {
		var r ServerInstanceRow
		if err := rows.Scan(&r.ID, &r.Name, &r.Region, &r.Host, &r.Port, &r.Active); err != nil {
			return nil, fmt.Errorf("scan server instance: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) CreateServerInstance(ctx context.Context, name, region, host string, port int) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO server_instances (name, region, host, port, active)
		VALUES ($1, NULLIF($2,''), $3, $4, TRUE)
	`, name, region, host, port)
	if err != nil {
		return fmt.Errorf("create server instance: %w", err)
	}
	return nil
}

func (s *Store) DeleteServerInstance(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM server_instances WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete server instance: %w", err)
	}
	return nil
}

func (s *Store) ResetUserSessions(ctx context.Context, userID int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE sessions
		SET disconnected_at = NOW()
		WHERE user_id = $1 AND disconnected_at IS NULL
	`, userID)
	if err != nil {
		return 0, fmt.Errorf("reset user sessions: %w", err)
	}
	rows, _ := res.RowsAffected()
	return rows, nil
}

func (s *Store) ListRecentPayments(ctx context.Context, limit int) ([]PaymentRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, amount::text, status, provider, TO_CHAR(created_at, 'YYYY-MM-DD HH24:MI:SS')
		FROM payments
		ORDER BY id DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list payments: %w", err)
	}
	defer rows.Close()
	out := make([]PaymentRow, 0, limit)
	for rows.Next() {
		var p PaymentRow
		if err := rows.Scan(&p.ID, &p.UserID, &p.Amount, &p.Status, &p.Provider, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) ListNotificationQueue(ctx context.Context, limit int) ([]NotificationRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, kind, status, TO_CHAR(scheduled_for, 'YYYY-MM-DD HH24:MI:SS')
		FROM notifications_queue
		ORDER BY id DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()
	out := make([]NotificationRow, 0, limit)
	for rows.Next() {
		var n NotificationRow
		if err := rows.Scan(&n.ID, &n.UserID, &n.Kind, &n.Status, &n.ScheduledFor); err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func tariffToMaxDevices(tariff string) int {
	switch tariff {
	case "pro":
		return 10
	case "standard":
		return 3
	default:
		return 1
	}
}
