package users

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"

	"sunkern.local/framework/db"
)

// usersController groups HTTP handlers for the /api/users resource.
type usersController struct {
	readDB  *sql.DB
	writeDB *sql.DB
}

func (c *usersController) list(w http.ResponseWriter, r *http.Request) {
	q := db.Select(&Users.TableInfo).
		Where(Users.DeletedAt.IsNull()).
		OrderBy(Users.CreatedAt.Asc())

	items, err := db.QueryAll[User](r.Context(), c.readDB, q)
	if err != nil {
		slog.ErrorContext(r.Context(), "users: list", "error", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": items,
	})
}
