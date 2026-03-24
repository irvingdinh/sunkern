package users

import (
	"database/sql"
	"log/slog"
	"net/http"

	"sunkern.local/framework/db"
	sunkernhttp "sunkern.local/framework/http"
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
		sunkernhttp.Error(w, err)
		return
	}

	sunkernhttp.JSON(w, http.StatusOK, items)
}
