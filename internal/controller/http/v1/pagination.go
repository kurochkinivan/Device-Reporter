package v1

type Pagination struct {
	Page       uint64 `json:"page"`
	Limit      uint64 `json:"limit"`
	Total      int    `json:"total"`
	TotalPages int    `json:"total_pages"`
}
