package usermod

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

type handler struct {
	db *sql.DB
}

func (h *handler) listUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, email, name, created_at, updated_at, deleted_at
		 FROM users
		 WHERE deleted_at IS NULL
		 ORDER BY created_at ASC`)
	if err != nil {
		slog.ErrorContext(r.Context(), "usermod: query users", "error", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		var u User
		var createdAt, updatedAt string
		var deletedAt sql.NullString
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &createdAt, &updatedAt, &deletedAt); err != nil {
			slog.ErrorContext(r.Context(), "usermod: scan user", "error", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
		u.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		u.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		if deletedAt.Valid {
			t, _ := time.Parse("2006-01-02 15:04:05", deletedAt.String)
			u.DeletedAt = &t
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(r.Context(), "usermod: iterate users", "error", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": users,
	})
}
