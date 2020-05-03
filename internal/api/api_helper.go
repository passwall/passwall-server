package api

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/pass-wall/passwall-server/internal/encryption"
)

// SetArgs ...
func SetArgs(r *http.Request, fields []string) (map[string]string, map[string]int) {

	// String type query params
	search := r.FormValue("Search")
	sort := r.FormValue("Sort")
	order := r.FormValue("Order")
	argsStr := map[string]string{
		"search": search,
		"order":  setOrder(fields, sort, order),
	}

	// Integer type query params
	offset := r.FormValue("Offset")
	limit := r.FormValue("Limit")
	argsInt := map[string]int{
		"offset": setOffset(offset),
		"limit":  setLimit(limit),
	}

	return argsStr, argsInt
}

// Offset returns the starting number of result for pagination
func setOffset(offset string) int {
	offsetInt, err := strconv.Atoi(offset)
	if err != nil {
		return -1
	}

	// don't allow negative values
	// except -1 which cancels offset condition
	if offsetInt < 0 {
		offsetInt = -1
	}
	return offsetInt
}

// Limit returns the number of result for pagination
func setLimit(limit string) int {
	limitInt, err := strconv.Atoi(limit)
	if err != nil {
		// -1 cancels limit condition
		return -1
	}

	// min limit should be 1
	if limitInt < 1 {
		limitInt = 1
	}
	return limitInt
}

// SortOrder returns the string for sorting and ordering data
func setOrder(fields []string, sort, order string) string {
	orderValues := []string{"desc", "asc"}

	if encryption.Include(fields, ToSnakeCase(sort)) && encryption.Include(orderValues, ToSnakeCase(order)) {
		return ToSnakeCase(sort) + " " + ToSnakeCase(order)
	}

	return "updated_at desc"
}

// ToSnakeCase changes string to database table
func ToSnakeCase(str string) string {
	var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")

	return strings.ToLower(snake)
}
