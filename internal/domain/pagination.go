package domain

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// Pagination is a 1-based page request.
type Pagination struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

// Normalize clamps Page to at least 1 and PageSize to [1, MaxPageSize],
// using DefaultPageSize when unset.
func (p *Pagination) Normalize() {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = DefaultPageSize
	}
	if p.PageSize > MaxPageSize {
		p.PageSize = MaxPageSize
	}
}

func (p Pagination) Limit() int {
	return p.PageSize
}

func (p Pagination) Offset() int {
	if p.Page < 1 {
		return 0
	}
	return (p.Page - 1) * p.PageSize
}

// Page is one page of results plus the total number of matching rows.
type Page[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

func NewPage[T any](items []T, total int64, p Pagination) Page[T] {
	if items == nil {
		items = []T{}
	}
	return Page[T]{Items: items, Total: total, Page: p.Page, PageSize: p.PageSize}
}
