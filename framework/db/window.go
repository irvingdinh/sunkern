package db

import (
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// Window definition builder — PARTITION BY, ORDER BY, frame clauses
// ---------------------------------------------------------------------------

// WindowDef defines a window specification for OVER clauses. Build one with
// Window() and configure with PartitionBy, OrderBy, and frame methods.
//
//	win := db.Window().
//	    PartitionBy(Orders.UserID).
//	    OrderBy(Orders.CreatedAt.Asc()).
//	    Rows(db.UnboundedPreceding, db.CurrentRow)
type WindowDef struct {
	partitionBy []Expr
	orderBy     []OrderExpr
	frameType   string // "ROWS", "RANGE", "GROUPS", or ""
	frameStart  frameBound
	frameEnd    frameBound
}

// Window creates a new empty window definition.
func Window() *WindowDef {
	return &WindowDef{}
}

// PartitionBy sets the PARTITION BY columns.
func (w *WindowDef) PartitionBy(cols ...Expr) *WindowDef {
	w.partitionBy = append(w.partitionBy, cols...)
	return w
}

// OrderBy sets the ORDER BY expressions for the window.
func (w *WindowDef) OrderBy(exprs ...OrderExpr) *WindowDef {
	w.orderBy = append(w.orderBy, exprs...)
	return w
}

// Rows sets a ROWS frame: ROWS BETWEEN start AND end.
//
//	db.Window().OrderBy(col.Asc()).Rows(db.UnboundedPreceding, db.CurrentRow)
func (w *WindowDef) Rows(start, end frameBound) *WindowDef {
	w.frameType = "ROWS"
	w.frameStart = start
	w.frameEnd = end
	return w
}

// Range sets a RANGE frame: RANGE BETWEEN start AND end.
//
//	db.Window().OrderBy(col.Asc()).Range(db.UnboundedPreceding, db.CurrentRow)
func (w *WindowDef) Range(start, end frameBound) *WindowDef {
	w.frameType = "RANGE"
	w.frameStart = start
	w.frameEnd = end
	return w
}

// Groups sets a GROUPS frame: GROUPS BETWEEN start AND end.
//
//	db.Window().OrderBy(col.Asc()).Groups(db.Preceding(2), db.Following(2))
func (w *WindowDef) Groups(start, end frameBound) *WindowDef {
	w.frameType = "GROUPS"
	w.frameStart = start
	w.frameEnd = end
	return w
}

// WriteSQL renders the window specification (without the OVER keyword).
func (w *WindowDef) WriteSQL(buf *strings.Builder, args *[]any) {
	buf.WriteString("(")
	needSpace := false

	if len(w.partitionBy) > 0 {
		buf.WriteString("PARTITION BY ")
		for i, col := range w.partitionBy {
			if i > 0 {
				buf.WriteString(", ")
			}
			col.WriteSQL(buf, args)
		}
		needSpace = true
	}

	if len(w.orderBy) > 0 {
		if needSpace {
			buf.WriteString(" ")
		}
		buf.WriteString("ORDER BY ")
		for i, o := range w.orderBy {
			if i > 0 {
				buf.WriteString(", ")
			}
			o.WriteSQL(buf, args)
		}
		needSpace = true
	}

	if w.frameType != "" {
		if needSpace {
			buf.WriteString(" ")
		}
		buf.WriteString(w.frameType)
		buf.WriteString(" BETWEEN ")
		buf.WriteString(w.frameStart.sql())
		buf.WriteString(" AND ")
		buf.WriteString(w.frameEnd.sql())
	}

	buf.WriteString(")")
}

// ---------------------------------------------------------------------------
// Frame bounds — UNBOUNDED PRECEDING, N PRECEDING, CURRENT ROW, etc.
// ---------------------------------------------------------------------------

// frameBound represents one side of a window frame (ROWS/RANGE/GROUPS BETWEEN).
type frameBound struct {
	kind  boundKind
	value int
}

type boundKind int

const (
	boundUnboundedPreceding boundKind = iota
	boundPreceding
	boundCurrentRow
	boundFollowing
	boundUnboundedFollowing
)

// UnboundedPreceding is UNBOUNDED PRECEDING — from the start of the partition.
var UnboundedPreceding = frameBound{kind: boundUnboundedPreceding}

// CurrentRow is CURRENT ROW.
var CurrentRow = frameBound{kind: boundCurrentRow}

// UnboundedFollowing is UNBOUNDED FOLLOWING — to the end of the partition.
var UnboundedFollowing = frameBound{kind: boundUnboundedFollowing}

// Preceding returns N PRECEDING — N rows/range before the current row.
func Preceding(n int) frameBound {
	return frameBound{kind: boundPreceding, value: n}
}

// Following returns N FOLLOWING — N rows/range after the current row.
func Following(n int) frameBound {
	return frameBound{kind: boundFollowing, value: n}
}

func (fb frameBound) sql() string {
	switch fb.kind {
	case boundUnboundedPreceding:
		return "UNBOUNDED PRECEDING"
	case boundPreceding:
		return strconv.Itoa(fb.value) + " PRECEDING"
	case boundCurrentRow:
		return "CURRENT ROW"
	case boundFollowing:
		return strconv.Itoa(fb.value) + " FOLLOWING"
	case boundUnboundedFollowing:
		return "UNBOUNDED FOLLOWING"
	default:
		return "CURRENT ROW"
	}
}

// ---------------------------------------------------------------------------
// Over — wraps any expression with an OVER clause
// ---------------------------------------------------------------------------

type overExpr struct {
	inner Expr
	win   *WindowDef
}

// Over wraps any expression with an OVER window clause. Use with aggregate
// functions to create window aggregates.
//
//	// Running total of order amounts, ordered by date:
//	db.As(
//	    db.Over(db.Sum(Orders.Amount, ""),
//	        db.Window().OrderBy(Orders.CreatedAt.Asc()).
//	            Rows(db.UnboundedPreceding, db.CurrentRow)),
//	    "running_total",
//	)
//
//	// Count per partition:
//	db.As(
//	    db.Over(db.CountAll(""), db.Window().PartitionBy(Orders.UserID)),
//	    "user_order_count",
//	)
func Over(expr Expr, win *WindowDef) Expr {
	return overExpr{inner: expr, win: win}
}

func (e overExpr) WriteSQL(buf *strings.Builder, args *[]any) {
	e.inner.WriteSQL(buf, args)
	buf.WriteString(" OVER ")
	e.win.WriteSQL(buf, args)
}

// ---------------------------------------------------------------------------
// Window-only functions — these have no non-window form
// ---------------------------------------------------------------------------

// RowNumber returns ROW_NUMBER() OVER(win). Assigns a unique sequential
// integer to each row within a partition.
//
//	db.As(
//	    db.RowNumber(db.Window().PartitionBy(Users.DeptID).OrderBy(Users.Salary.Desc())),
//	    "rank",
//	)
func RowNumber(win *WindowDef) Expr {
	return windowFuncExpr{fn: "ROW_NUMBER", win: win}
}

// Rank returns RANK() OVER(win). Like ROW_NUMBER but rows with equal
// ORDER BY values get the same rank, with gaps after ties.
func Rank(win *WindowDef) Expr {
	return windowFuncExpr{fn: "RANK", win: win}
}

// DenseRank returns DENSE_RANK() OVER(win). Like RANK but without gaps
// after ties.
func DenseRank(win *WindowDef) Expr {
	return windowFuncExpr{fn: "DENSE_RANK", win: win}
}

// NTile returns NTILE(n) OVER(win). Divides the partition into n roughly
// equal groups and assigns the group number (1-based) to each row.
func NTile(n int, win *WindowDef) Expr {
	return windowFuncExpr{fn: "NTILE", arg: strconv.Itoa(n), win: win}
}

// Lag returns LAG(expr, offset) OVER(win). Returns the value of expr from
// the row that is offset rows before the current row within the partition.
// Returns NULL if no such row exists.
//
//	db.As(
//	    db.Lag(Orders.Amount, 1, db.Window().OrderBy(Orders.CreatedAt.Asc())),
//	    "prev_amount",
//	)
func Lag(expr Expr, offset int, win *WindowDef) Expr {
	return windowOffsetExpr{fn: "LAG", inner: expr, offset: offset, win: win}
}

// Lead returns LEAD(expr, offset) OVER(win). Returns the value of expr from
// the row that is offset rows after the current row within the partition.
// Returns NULL if no such row exists.
func Lead(expr Expr, offset int, win *WindowDef) Expr {
	return windowOffsetExpr{fn: "LEAD", inner: expr, offset: offset, win: win}
}

// LagDefault returns LAG(expr, offset, default) OVER(win). Like Lag but
// returns the default value instead of NULL when no prior row exists.
func LagDefault(expr Expr, offset int, defaultVal Expr, win *WindowDef) Expr {
	return windowOffsetDefaultExpr{fn: "LAG", inner: expr, offset: offset, defaultVal: defaultVal, win: win}
}

// LeadDefault returns LEAD(expr, offset, default) OVER(win). Like Lead but
// returns the default value instead of NULL when no subsequent row exists.
func LeadDefault(expr Expr, offset int, defaultVal Expr, win *WindowDef) Expr {
	return windowOffsetDefaultExpr{fn: "LEAD", inner: expr, offset: offset, defaultVal: defaultVal, win: win}
}

// FirstValue returns FIRST_VALUE(expr) OVER(win). Returns the value of expr
// evaluated at the first row of the window frame.
func FirstValue(expr Expr, win *WindowDef) Expr {
	return windowValueExpr{fn: "FIRST_VALUE", inner: expr, win: win}
}

// LastValue returns LAST_VALUE(expr) OVER(win). Returns the value of expr
// evaluated at the last row of the window frame. Note: the default frame
// is RANGE BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW, so use an explicit
// frame to include all rows:
//
//	db.LastValue(col, db.Window().OrderBy(col.Asc()).
//	    Rows(db.UnboundedPreceding, db.UnboundedFollowing))
func LastValue(expr Expr, win *WindowDef) Expr {
	return windowValueExpr{fn: "LAST_VALUE", inner: expr, win: win}
}

// NthValue returns NTH_VALUE(expr, n) OVER(win). Returns the value of expr
// evaluated at the nth row of the window frame (1-based).
func NthValue(expr Expr, n int, win *WindowDef) Expr {
	return windowNthExpr{inner: expr, n: n, win: win}
}

// ---------------------------------------------------------------------------
// Internal types for window function SQL generation
// ---------------------------------------------------------------------------

// windowFuncExpr: FUNC() OVER(...) or FUNC(arg) OVER(...)
type windowFuncExpr struct {
	fn  string
	arg string // optional static argument (e.g., "4" for NTILE)
	win *WindowDef
}

func (e windowFuncExpr) WriteSQL(buf *strings.Builder, args *[]any) {
	buf.WriteString(e.fn)
	buf.WriteString("(")
	if e.arg != "" {
		buf.WriteString(e.arg)
	}
	buf.WriteString(") OVER ")
	e.win.WriteSQL(buf, args)
}

// windowOffsetExpr: FUNC(expr, offset) OVER(...)
type windowOffsetExpr struct {
	fn     string
	inner  Expr
	offset int
	win    *WindowDef
}

func (e windowOffsetExpr) WriteSQL(buf *strings.Builder, args *[]any) {
	buf.WriteString(e.fn)
	buf.WriteString("(")
	e.inner.WriteSQL(buf, args)
	buf.WriteString(", ")
	buf.WriteString(strconv.Itoa(e.offset))
	buf.WriteString(") OVER ")
	e.win.WriteSQL(buf, args)
}

// windowOffsetDefaultExpr: FUNC(expr, offset, default) OVER(...)
type windowOffsetDefaultExpr struct {
	fn         string
	inner      Expr
	offset     int
	defaultVal Expr
	win        *WindowDef
}

func (e windowOffsetDefaultExpr) WriteSQL(buf *strings.Builder, args *[]any) {
	buf.WriteString(e.fn)
	buf.WriteString("(")
	e.inner.WriteSQL(buf, args)
	buf.WriteString(", ")
	buf.WriteString(strconv.Itoa(e.offset))
	buf.WriteString(", ")
	e.defaultVal.WriteSQL(buf, args)
	buf.WriteString(") OVER ")
	e.win.WriteSQL(buf, args)
}

// windowValueExpr: FUNC(expr) OVER(...)
type windowValueExpr struct {
	fn    string
	inner Expr
	win   *WindowDef
}

func (e windowValueExpr) WriteSQL(buf *strings.Builder, args *[]any) {
	buf.WriteString(e.fn)
	buf.WriteString("(")
	e.inner.WriteSQL(buf, args)
	buf.WriteString(") OVER ")
	e.win.WriteSQL(buf, args)
}

// windowNthExpr: NTH_VALUE(expr, n) OVER(...)
type windowNthExpr struct {
	inner Expr
	n     int
	win   *WindowDef
}

func (e windowNthExpr) WriteSQL(buf *strings.Builder, args *[]any) {
	buf.WriteString("NTH_VALUE(")
	e.inner.WriteSQL(buf, args)
	buf.WriteString(", ")
	buf.WriteString(strconv.Itoa(e.n))
	buf.WriteString(") OVER ")
	e.win.WriteSQL(buf, args)
}
