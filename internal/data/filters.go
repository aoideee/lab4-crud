// internal/data/filters.go
package data

import (
	"math"
	"strings"
)

// Filters holds pagination and sorting parameters extracted from URL query strings.
type Filters struct {
	Page         int      // Current page number (1-indexed)
	PageSize     int      // Number of records per page
	Sort         string   // Column name to sort by (prefix with "-" for DESC)
	SortSafeList []string // Allowed sort columns to prevent SQL injection
}

// sortColumn returns the validated column name for ORDER BY, defaulting to book_id.
func (f Filters) sortColumn() string {
	for _, safe := range f.SortSafeList {
		if f.Sort == safe {
			return strings.TrimPrefix(f.Sort, "-")
		}
	}
	return "book_id" // safe fallback
}

// sortDirection returns "ASC" or "DESC" based on the Sort prefix.
func (f Filters) sortDirection() string {
	if strings.HasPrefix(f.Sort, "-") {
		return "DESC"
	}
	return "ASC"
}

// limit returns the SQL LIMIT value derived from PageSize.
func (f Filters) limit() int { return f.PageSize }

// offset returns the SQL OFFSET value derived from Page and PageSize.
func (f Filters) offset() int { return (f.Page - 1) * f.PageSize }

// Metadata contains pagination information returned alongside list responses.
type Metadata struct {
	CurrentPage  int `json:"current_page,omitempty"`
	PageSize     int `json:"page_size,omitempty"`
	FirstPage    int `json:"first_page,omitempty"`
	LastPage     int `json:"last_page,omitempty"`
	TotalRecords int `json:"total_records,omitempty"`
}

// calculateMetadata computes page metadata from total record count and filter values.
func calculateMetadata(totalRecords, page, pageSize int) Metadata {
	if totalRecords == 0 {
		return Metadata{}
	}
	return Metadata{
		CurrentPage:  page,
		PageSize:     pageSize,
		FirstPage:    1,
		LastPage:     int(math.Ceil(float64(totalRecords) / float64(pageSize))),
		TotalRecords: totalRecords,
	}
}
