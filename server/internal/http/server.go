package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"teledrive/server/internal/storage"
)

type Store interface {
	UpsertUserByTelegramID(ctx context.Context, telegramID int64) (storage.User, error)
	CreateOrRefreshSession(ctx context.Context, userID int64, deviceID, ip string) error
	DisconnectSession(ctx context.Context, userID int64, deviceID string) error
	CountActiveSessions(ctx context.Context, userID int64) (int, error)
	GetOrCreateActiveConfig(ctx context.Context, userID int64) (string, error)
	SetConfigMode(ctx context.Context, userID int64, mode string) error
	LogAction(ctx context.Context, userID int64, action string, payload string) error
}

type PaymentService interface {
	ConfirmPayment(ctx context.Context, paymentID int64) error
}

type Limiter interface {
	AllowConnection(ctx context.Context, userID int64, maxDevices int) (bool, int64, error)
	ReleaseConnection(ctx context.Context, userID int64) error
}

type WGBuilder interface {
	BuildClientConfig(userID int64, privateKey string) string
}

// Server — HTTP API с health/readiness и endpoint'ами сессий.
type Server struct {
	httpServer     *http.Server
	mux            *http.ServeMux
	store          Store
	limiter        Limiter
	wgBuilder      WGBuilder
	paymentService PaymentService
}

type connectRequest struct {
	TelegramID int64  `json:"telegram_id"`
	DeviceID   string `json:"device_id"`
	IP         string `json:"ip"`
}

type disconnectRequest struct {
	TelegramID int64  `json:"telegram_id"`
	DeviceID   string `json:"device_id"`
}

type generateConfigRequest struct {
	TelegramID int64  `json:"telegram_id"`
	Mode       string `json:"mode"`
}

type confirmPaymentRequest struct {
	PaymentID int64 `json:"payment_id"`
}

func New(addr string, store Store, limiter Limiter, paymentService PaymentService, wgBuilder WGBuilder) *Server {
	s := &Server{store: store, limiter: limiter, paymentService: paymentService, wgBuilder: wgBuilder}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/api/v1/sessions/connect", s.handleConnect)
	mux.HandleFunc("/api/v1/sessions/disconnect", s.handleDisconnect)
	mux.HandleFunc("/api/v1/configs/generate", s.handleGenerateConfig)
	mux.HandleFunc("/api/v1/payments/confirm", s.handleConfirmPayment)

	s.mux = mux
	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return s
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) { /* unchanged */
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req connectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.TelegramID == 0 || req.DeviceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "telegram_id and device_id are required"})
		return
	}
	ctx := r.Context()
	user, err := s.store.UpsertUserByTelegramID(ctx, req.TelegramID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "user lookup failed"})
		return
	}
	allowed, current, err := s.limiter.AllowConnection(ctx, user.ID, user.MaxDevices)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "rate limiter failed"})
		return
	}
	if !allowed {
		_ = s.store.LogAction(ctx, user.ID, "session_rejected", req.DeviceID)
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "device limit exceeded", "max_devices": user.MaxDevices, "active": current})
		return
	}
	if err := s.store.CreateOrRefreshSession(ctx, user.ID, req.DeviceID, req.IP); err != nil {
		_ = s.limiter.ReleaseConnection(ctx, user.ID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "session persist failed"})
		return
	}
	_ = s.store.LogAction(ctx, user.ID, "session_connected", req.DeviceID)
	active, _ := s.store.CountActiveSessions(ctx, user.ID)
	writeJSON(w, http.StatusOK, map[string]any{"status": "connected", "max_devices": user.MaxDevices, "active": active})
}

func (s *Server) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req disconnectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.TelegramID == 0 || req.DeviceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "telegram_id and device_id are required"})
		return
	}
	ctx := r.Context()
	user, err := s.store.UpsertUserByTelegramID(ctx, req.TelegramID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "user lookup failed"})
		return
	}
	if err := s.store.DisconnectSession(ctx, user.ID, req.DeviceID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "disconnect failed"})
		return
	}
	if err := s.limiter.ReleaseConnection(ctx, user.ID); err != nil && !errors.Is(err, context.Canceled) {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "limiter release failed"})
		return
	}
	_ = s.store.LogAction(ctx, user.ID, "session_disconnected", req.DeviceID)
	active, _ := s.store.CountActiveSessions(ctx, user.ID)
	writeJSON(w, http.StatusOK, map[string]any{"status": "disconnected", "active": active})
}

func (s *Server) handleGenerateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req generateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.TelegramID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "telegram_id is required"})
		return
	}
	ctx := r.Context()
	user, err := s.store.UpsertUserByTelegramID(ctx, req.TelegramID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "user lookup failed"})
		return
	}
	mode := req.Mode
	if mode == "" {
		mode = "full_tunnel"
	}
	if mode != "full_tunnel" && mode != "split_tunnel" && mode != "per_app" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid mode"})
		return
	}
	if err := s.store.SetConfigMode(ctx, user.ID, mode); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "set mode failed"})
		return
	}
	privateKey, err := s.store.GetOrCreateActiveConfig(ctx, user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "load config failed"})
		return
	}
	cfgText := s.wgBuilder.BuildClientConfig(user.ID, privateKey)
	_ = s.store.LogAction(ctx, user.ID, "config_generated", mode)
	writeJSON(w, http.StatusOK, map[string]any{"mode": mode, "config": cfgText})
}

func (s *Server) handleConfirmPayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req confirmPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.PaymentID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payment_id is required"})
		return
	}
	if err := s.paymentService.ConfirmPayment(r.Context(), req.PaymentID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "confirm payment failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "paid"})
}

type RouteRegistrar interface {
	Register(mux *http.ServeMux)
}

func (s *Server) RegisterModule(r RouteRegistrar) {
	if r != nil {
		r.Register(s.mux)
	}
}

func (s *Server) Start() error                       { return s.httpServer.ListenAndServe() }
func (s *Server) Shutdown(ctx context.Context) error { return s.httpServer.Shutdown(ctx) }

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
