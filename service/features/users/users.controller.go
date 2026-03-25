package users

import (
	"database/sql"
	"errors"
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

// listRequest holds query parameters for the list endpoint.
type listRequest struct {
	sunkernhttp.PaginationParams
}

func (c *usersController) list(w http.ResponseWriter, r *http.Request) {
	var params listRequest
	if err := sunkernhttp.BindQuery(r, &params); err != nil {
		sunkernhttp.Error(w, err)
		return
	}
	page, perPage, offset := params.Paginate()

	q := db.Select(&Users.TableInfo).
		Apply(db.NotDeleted(Users.DeletedAt)).
		OrderBy(Users.CreatedAt.Desc())

	total, err := db.Count(r.Context(), c.readDB, q)
	if err != nil {
		slog.ErrorContext(r.Context(), "users: count", "error", err)
		sunkernhttp.Error(w, err)
		return
	}

	q = q.Apply(db.Paginate(perPage, offset))

	items, err := db.QueryAll[User](r.Context(), c.readDB, q)
	if err != nil {
		slog.ErrorContext(r.Context(), "users: list", "error", err)
		sunkernhttp.Error(w, err)
		return
	}

	sunkernhttp.JSONList(w, items, int(total), page, perPage)
}

func (c *usersController) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	user, err := db.FindByID[User](r.Context(), c.readDB, &Users.TableInfo, id, db.NotDeleted(Users.DeletedAt))
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			sunkernhttp.Error(w, sunkernhttp.ErrNotFound)
			return
		}
		slog.ErrorContext(r.Context(), "users: get", "error", err)
		sunkernhttp.Error(w, err)
		return
	}

	sunkernhttp.JSON(w, http.StatusOK, user)
}

type createRequest struct {
	Email string `json:"email" validate:"required,email"`
	Name  string `json:"name"`
}

func (c *usersController) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := sunkernhttp.Bind(r, &req); err != nil {
		sunkernhttp.Error(w, err)
		return
	}

	// Check for duplicate email.
	exists, err := db.Exists(r.Context(), c.readDB,
		db.Select(&Users.TableInfo).
			Where(Users.Email.Eq(req.Email)).
			Apply(db.NotDeleted(Users.DeletedAt)))
	if err != nil {
		slog.ErrorContext(r.Context(), "users: check duplicate", "error", err)
		sunkernhttp.Error(w, err)
		return
	}
	if exists {
		sunkernhttp.Error(w, sunkernhttp.ErrConflict.WithMessage("email already in use"))
		return
	}

	user := User{Email: req.Email, Name: req.Name}
	if _, err := db.Insert(&Users.TableInfo).Model(&user).Exec(r.Context(), c.writeDB); err != nil {
		slog.ErrorContext(r.Context(), "users: create", "error", err)
		sunkernhttp.Error(w, err)
		return
	}

	sunkernhttp.Created(w, user)
}

type updateRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (c *usersController) update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req updateRequest
	if err := sunkernhttp.Bind(r, &req); err != nil {
		sunkernhttp.Error(w, err)
		return
	}

	ub := db.UpdateByID(&Users.TableInfo, id)

	if req.Email != "" {
		ub = ub.Set(Users.Email, req.Email)
	}
	if req.Name != "" {
		ub = ub.Set(Users.Name, req.Name)
	}

	result, err := ub.Exec(r.Context(), c.writeDB)
	if err != nil {
		slog.ErrorContext(r.Context(), "users: update", "error", err)
		sunkernhttp.Error(w, err)
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		sunkernhttp.Error(w, sunkernhttp.ErrNotFound)
		return
	}

	// Fetch and return the updated user.
	user, err := db.FindByID[User](r.Context(), c.readDB, &Users.TableInfo, id)
	if err != nil {
		slog.ErrorContext(r.Context(), "users: get after update", "error", err)
		sunkernhttp.Error(w, err)
		return
	}

	sunkernhttp.JSON(w, http.StatusOK, user)
}

func (c *usersController) delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	result, err := db.SoftDeleteByID(r.Context(), c.writeDB, &Users.TableInfo, id)
	if err != nil {
		slog.ErrorContext(r.Context(), "users: delete", "error", err)
		sunkernhttp.Error(w, err)
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		sunkernhttp.Error(w, sunkernhttp.ErrNotFound)
		return
	}

	sunkernhttp.NoContent(w)
}

func (c *usersController) restore(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	result, err := db.RestoreByID(r.Context(), c.writeDB, &Users.TableInfo, id)
	if err != nil {
		slog.ErrorContext(r.Context(), "users: restore", "error", err)
		sunkernhttp.Error(w, err)
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		sunkernhttp.Error(w, sunkernhttp.ErrNotFound)
		return
	}

	// Fetch and return the restored user.
	user, err := db.FindByID[User](r.Context(), c.readDB, &Users.TableInfo, id)
	if err != nil {
		slog.ErrorContext(r.Context(), "users: get after restore", "error", err)
		sunkernhttp.Error(w, err)
		return
	}

	sunkernhttp.JSON(w, http.StatusOK, user)
}
