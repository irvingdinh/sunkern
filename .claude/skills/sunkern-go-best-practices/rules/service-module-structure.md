---
title: Service Module Structure
impact: HIGH
impactDescription: Incorrect module structure leads to circular dependencies, naming conflicts, import path chaos, and inconsistent code that AI agents cannot maintain mechanically.
tags: module, structure, controller, entity, naming, convention, feature
---

## Service Module Structure

**Location:** `service/features/{feature-name}/`

Sunkern service modules follow a flat-package, NestJS-inspired file naming convention. Each feature is ONE Go package — no sub-packages for controllers, entities, or services. Role identification is via file name suffix, not subdirectory.

---

### File Naming Convention

Every module follows this recipe. File names use dot-separated suffixes to identify their role:

| File Pattern | Role | Required? |
|-------------|------|-----------|
| `{feature}.module.go` | Module lifecycle (Register/Boot/Shutdown), route wiring | Always |
| `{feature}.controller.go` | HTTP handlers for the primary resource | Always |
| `{sub-resource}.controller.go` | HTTP handlers for sub-resources | When sub-resources exist |
| `{entity-singular}.entity.go` | Domain model struct + table schema definition | Always |
| `{feature}.service.go` | Shared business logic extracted from controllers | When controllers share logic or exceed ~30 lines per method |
| `{feature}.scope.go` | Reusable query scopes | When reusable query patterns emerge |

**File names use lowercase with hyphens for multi-word names:** `user-settings.controller.go`, not `userSettings.controller.go` or `user_settings.controller.go`.

---

### Directory Naming Convention

| Element | Convention | Examples |
|---------|-----------|---------|
| Feature directory | Plural, lowercase, hyphens for multi-word | `users/`, `orders/`, `api-keys/` |
| Go package name | Plural, lowercase, NO hyphens (Go constraint) | `package users`, `package orders`, `package apikeys` |
| Sub-module directory | Action-noun or resource name | `admin/auth/`, `admin/user-mgmt/` |

**The directory name matches the REST resource.** `features/users/` serves `/api/users`. `features/orders/` serves `/api/orders`.

**Drop the `mod` suffix.** Use `users`, not `usermod`. The directory location (`features/`) already establishes these are modules.

---

### Module File: `{feature}.module.go`

The composition root. Resolves dependencies, creates controllers, registers routes.

```go
package users

import (
    "sunkern.local/framework/app"
    "sunkern.local/framework/container"
    sunkernhttp "sunkern.local/framework/http"
    "sunkern.local/framework/sqlite"
)

// Module is the users feature module.
type Module struct {
    app.BaseModule
}

// New creates the users module.
func New() *Module {
    return &Module{
        BaseModule: app.BaseModule{ModuleName: "users"},
    }
}

// Boot resolves dependencies and registers routes.
func (m *Module) Boot() error {
    server := container.MustMake[sunkernhttp.Server]()
    sqliteDB := container.MustMake[*sqlite.DB]()

    uc := &usersController{
        readDB:  sqliteDB.ReadDB(),
        writeDB: sqliteDB.WriteDB(),
    }
    server.Mux().HandleFunc("GET /api/users", uc.list)
    server.Mux().HandleFunc("POST /api/users", uc.create)
    server.Mux().HandleFunc("GET /api/users/{id}", uc.show)
    server.Mux().HandleFunc("PUT /api/users/{id}", uc.update)
    server.Mux().HandleFunc("DELETE /api/users/{id}", uc.delete)

    // Sub-resource controllers are wired here too.
    sc := &settingsController{readDB: sqliteDB.ReadDB(), writeDB: sqliteDB.WriteDB()}
    server.Mux().HandleFunc("GET /api/users/{id}/settings", sc.show)
    server.Mux().HandleFunc("PUT /api/users/{id}/settings", sc.update)

    return nil
}
```

**Rules:**
- All route registration happens in `Boot()`, nowhere else.
- Each controller struct is created in `Boot()` with its dependencies.
- `ModuleName` matches the package name (e.g., `"users"`).

---

### Controller File: `{resource}.controller.go`

Groups all HTTP handlers for one REST resource.

```go
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
    json.NewEncoder(w).Encode(map[string]any{"data": items})
}

func (c *usersController) show(w http.ResponseWriter, r *http.Request) { /* ... */ }
func (c *usersController) create(w http.ResponseWriter, r *http.Request) { /* ... */ }
func (c *usersController) update(w http.ResponseWriter, r *http.Request) { /* ... */ }
func (c *usersController) delete(w http.ResponseWriter, r *http.Request) { /* ... */ }
```

**Rules:**
- Controller struct names follow the pattern `{resource}Controller` — **unexported** (private to the package).
- The primary resource controller is named after the feature: `usersController`.
- Sub-resource controllers use the sub-resource name: `settingsController`, `addressesController`.
- Controller methods are named after the HTTP action: `list`, `show`, `create`, `update`, `delete`.
- Controllers hold `readDB` and `writeDB` — use `readDB` for queries, `writeDB` for mutations.
- Use `slog.ErrorContext` for logging (never `fmt.Println`).

---

### Entity File: `{singular}.entity.go`

Contains BOTH the domain model struct and the typed table schema. They are always 1:1 in Sunkern.

```go
package users

import "sunkern.local/framework/db"

// ---------------------------------------------------------------------------
// Domain Model
// ---------------------------------------------------------------------------

// User represents a user account.
type User struct {
    db.BaseModel
    Email string `db:"email" json:"email"`
    Name  string `db:"name"  json:"name"`
}

// ---------------------------------------------------------------------------
// Table Schema
// ---------------------------------------------------------------------------

type usersTable struct {
    db.TableInfo
    ID        db.StringColumn
    Email     db.StringColumn
    Name      db.StringColumn
    CreatedAt db.TimeColumn
    UpdatedAt db.TimeColumn
    DeletedAt db.NullTimeColumn
}

// Users is the typed table reference for the users table.
var Users = func() usersTable {
    t := usersTable{TableInfo: db.NewTableInfo("users")}
    t.ID        = db.String(&t.TableInfo, "id")
    t.Email     = db.String(&t.TableInfo, "email")
    t.Name      = db.String(&t.TableInfo, "name")
    t.CreatedAt = db.Time(&t.TableInfo, "created_at")
    t.UpdatedAt = db.Time(&t.TableInfo, "updated_at")
    t.DeletedAt = db.NullTime(&t.TableInfo, "deleted_at")
    return t
}()
```

**Rules:**
- Entity file is named with the **singular** form: `user.entity.go`, `order.entity.go`.
- The model struct is **exported** and **singular**: `User`, `Order`, `Product`.
- The table struct is **unexported**: `usersTable`, `ordersTable`.
- The table variable is **exported** and **plural**: `var Users`, `var Orders`.
- Every model embeds `db.BaseModel` as the first field (provides ID, CreatedAt, UpdatedAt, DeletedAt).
- Both `db:"..."` and `json:"..."` tags are required on every field.
- Table columns use the exact SQL column names from the migration.

---

### Sub-Resource Controllers

When a resource has nested sub-resources (e.g., `/api/users/:id/settings`), create a separate controller file in the SAME package:

```
features/users/
    users.module.go
    users.controller.go           # /api/users
    user-settings.controller.go   # /api/users/:id/settings
    user.entity.go
```

The sub-resource controller is a separate struct in the same package:

```go
// user-settings.controller.go
package users

type settingsController struct {
    readDB  *sql.DB
    writeDB *sql.DB
}

func (c *settingsController) show(w http.ResponseWriter, r *http.Request) {
    userID := r.PathValue("id")
    // ...
}
```

**Do NOT create a separate module for sub-resources** unless they have genuinely independent lifecycle (rare).

---

### Hierarchical Modules (Sub-Modules)

For complex modules like `admin` that contain multiple management areas, use Go sub-packages as sub-modules:

```
features/admin/
    admin.module.go              # ModuleGroup composing sub-modules

features/admin/auth/
    auth.module.go
    auth.controller.go
    session.entity.go

features/admin/user-mgmt/
    user-mgmt.module.go
    users.controller.go
```

The parent module uses `ModuleGroup`:

```go
// features/admin/admin.module.go
package admin

import (
    "sunkern.local/framework/app"
    "sunkern.local/service/features/admin/auth"
    usermgmt "sunkern.local/service/features/admin/user-mgmt"
)

func New() *app.ModuleGroup {
    return &app.ModuleGroup{
        GroupName: "admin",
        Modules: []app.Module{
            auth.New(),
            usermgmt.New(),
        },
    }
}
```

**Rules:**
- Nest by **domain** (sub-feature), never by **layer** (controller/entity).
- Each sub-module is its own Go package with its own `.module.go`.
- Sub-modules communicate through the container, not direct imports between siblings.

---

### Cross-Feature References

When one feature needs types from another (e.g., orders need `users.User`):

```go
// features/orders/order.entity.go
package orders

import "sunkern.local/service/features/users"

type Order struct {
    db.BaseModel
    UserID string      `db:"user_id" json:"user_id"`
    Total  float64     `db:"total"   json:"total"`
    User   *users.User `db:"-"       json:"user,omitempty"`
}
```

**Rules:**
- Import the other feature's package directly — the types are exported.
- Dependency direction must be one-way. If A imports B, B cannot import A.
- If circular dependency emerges, extract shared types into `features/shared/`.
- The `features/shared/` package must contain ONLY type definitions — zero business logic.

---

### What NOT to Do

| Anti-pattern | Why | Do this instead |
|-------------|-----|-----------------|
| Sub-directories for layers (`entities/`, `controllers/`) | Creates generic package names, stuttering (`entities.UserEntity`), circular dep risk | Flat package with file suffixes |
| `mod` suffix on package names (`usermod`) | Unnecessary noise, not REST-aligned | Plain plural name (`users`) |
| Central routes file | Scatters route registration away from handlers | Register routes in each module's `Boot()` |
| God file with all handlers | Hard to navigate at scale | One controller file per REST resource |
| Separate model and table files | They're always 1:1, splitting provides no benefit | Single entity file with both |
| `internal/` directories inside features | Adds import path depth for no encapsulation benefit | Everything in the flat feature package |
| `utils/`, `helpers/`, `common/` packages | Go anti-pattern: meaningless names, dependency magnets | Put shared logic in the feature that owns it |

---

### AI Agent Recipe: Adding a New Feature

1. Create `service/features/{plural-name}/`
2. Create `{name}.module.go` — Module struct, `New()`, `Boot()` with route registration
3. Create `{name}.controller.go` — `{name}Controller` struct with handler methods
4. Create `{singular}.entity.go` — domain model struct + table schema
5. Create migration: `service/migrations/NNNNN_{description}.sql`
6. Register in `service/main.go`: `a.Use({name}.New())`

This recipe is identical for every feature. No architectural decisions required.
