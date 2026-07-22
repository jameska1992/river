package services

func paginationOffsetLimit(page, limit int) (offset, lim int) {
	lim = 50
	if limit > 0 && limit <= 200 {
		lim = limit
	}
	if page > 1 {
		offset = (page - 1) * lim
	}
	return
}

func defaultJSON(s string) string {
	if s == "" {
		return "[]"
	}
	return s
}
