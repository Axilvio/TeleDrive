package admin

import (
	"context"
	"embed"
	"html/template"
	"net/http"
	"strconv"

	"teledrive/server/internal/storage"
)

type Store interface {
	AdminStats(ctx context.Context) (storage.AdminStats, error)
	ListUsers(ctx context.Context, limit int) ([]storage.UserRow, error)
	ListServerInstances(ctx context.Context, limit int) ([]storage.ServerInstanceRow, error)
	CreateServerInstance(ctx context.Context, name, region, host string, port int) error
	DeleteServerInstance(ctx context.Context, id int64) error
	ResetUserSessions(ctx context.Context, userID int64) (int64, error)
	ListRecentPayments(ctx context.Context, limit int) ([]storage.PaymentRow, error)
	ListNotificationQueue(ctx context.Context, limit int) ([]storage.NotificationRow, error)
}

//go:embed templates/*.html
var templatesFS embed.FS

type Handler struct {
	store      Store
	templates  *template.Template
	adminToken string
}

func NewHandler(store Store, adminToken string) (*Handler, error) {
	tpls, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Handler{store: store, templates: tpls, adminToken: adminToken}, nil
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/admin", h.dashboard)
	mux.HandleFunc("/admin/users", h.usersTable)
	mux.HandleFunc("/admin/instances", h.instancesTable)
	mux.HandleFunc("/admin/instances/create", h.createInstance)
	mux.HandleFunc("/admin/instances/delete", h.deleteInstance)
	mux.HandleFunc("/admin/sessions/reset", h.resetSessions)
	mux.HandleFunc("/admin/payments", h.paymentsTable)
	mux.HandleFunc("/admin/notifications", h.notificationsTable)
}

func (h *Handler) authorize(r *http.Request) bool {
	if h.adminToken == "" {
		return true
	}
	return r.Header.Get("X-Admin-Token") == h.adminToken
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	stats, err := h.store.AdminStats(r.Context())
	if err != nil {
		http.Error(w, "failed to load stats", http.StatusInternalServerError)
		return
	}
	users, _ := h.store.ListUsers(r.Context(), 20)
	instances, _ := h.store.ListServerInstances(r.Context(), 20)
	payments, _ := h.store.ListRecentPayments(r.Context(), 20)
	notifyQ, _ := h.store.ListNotificationQueue(r.Context(), 20)
	data := map[string]any{"Stats": stats, "Users": users, "Instances": instances, "Payments": payments, "Notifications": notifyQ}
	if err := h.templates.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		http.Error(w, "template render error", http.StatusInternalServerError)
	}
}

func (h *Handler) usersTable(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	users, err := h.store.ListUsers(r.Context(), parseLimit(r, 50))
	if err != nil {
		http.Error(w, "failed to load users", http.StatusInternalServerError)
		return
	}
	_ = h.templates.ExecuteTemplate(w, "users_table.html", map[string]any{"Users": users})
}

func (h *Handler) instancesTable(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	rows, err := h.store.ListServerInstances(r.Context(), parseLimit(r, 50))
	if err != nil {
		http.Error(w, "failed to load instances", http.StatusInternalServerError)
		return
	}
	_ = h.templates.ExecuteTemplate(w, "instances_table.html", map[string]any{"Instances": rows})
}

func (h *Handler) createInstance(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	port, _ := strconv.Atoi(r.FormValue("port"))
	if port <= 0 {
		port = 443
	}
	if err := h.store.CreateServerInstance(r.Context(), r.FormValue("name"), r.FormValue("region"), r.FormValue("host"), port); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}
	h.instancesTable(w, r)
}

func (h *Handler) deleteInstance(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	_ = h.store.DeleteServerInstance(r.Context(), id)
	h.instancesTable(w, r)
}

func (h *Handler) resetSessions(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, _ := strconv.ParseInt(r.FormValue("user_id"), 10, 64)
	_, _ = h.store.ResetUserSessions(r.Context(), userID)
	h.usersTable(w, r)
}

func (h *Handler) paymentsTable(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	rows, err := h.store.ListRecentPayments(r.Context(), parseLimit(r, 50))
	if err != nil {
		http.Error(w, "failed to load payments", http.StatusInternalServerError)
		return
	}
	_ = h.templates.ExecuteTemplate(w, "payments_table.html", map[string]any{"Payments": rows})
}

func (h *Handler) notificationsTable(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	rows, err := h.store.ListNotificationQueue(r.Context(), parseLimit(r, 50))
	if err != nil {
		http.Error(w, "failed to load notifications", http.StatusInternalServerError)
		return
	}
	_ = h.templates.ExecuteTemplate(w, "notifications_table.html", map[string]any{"Notifications": rows})
}

func parseLimit(r *http.Request, fallback int) int {
	limit := fallback
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}
	return limit
}
