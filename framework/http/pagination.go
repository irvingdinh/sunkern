package http

// DefaultPerPage is the per-page size when the client omits or sends 0.
const DefaultPerPage = 20

// MaxPerPage is the maximum per-page size. Larger values are clamped.
const MaxPerPage = 100

// PaginationParams provides standard page/per_page query parameters.
// Embed in list request structs to eliminate pagination boilerplate.
// Call [PaginationParams.Paginate] to get clamped values and SQL offset.
//
//	type listRequest struct {
//	    sunkernhttp.PaginationParams
//	    Status string `query:"status"`
//	}
//
//	var params listRequest
//	if err := sunkernhttp.BindQuery(r, &params); err != nil { ... }
//	page, perPage, offset := params.Paginate()
type PaginationParams struct {
	Page    int `query:"page"`
	PerPage int `query:"per_page"`
}

// Paginate clamps Page and PerPage to sensible defaults and returns the
// clamped values plus the calculated SQL offset. Page starts at 1.
// PerPage defaults to [DefaultPerPage] (20), capped at [MaxPerPage] (100).
func (p PaginationParams) Paginate() (page, perPage, offset int) {
	page = p.Page
	if page <= 0 {
		page = 1
	}
	perPage = p.PerPage
	if perPage <= 0 || perPage > MaxPerPage {
		perPage = DefaultPerPage
	}
	offset = (page - 1) * perPage
	return
}
