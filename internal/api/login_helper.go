package api

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
	"github.com/pass-wall/passwall-server/internal/encryption"
	"github.com/spf13/viper"
)

// SetArgs ...
func SetArgs(r *http.Request) (map[string]string, map[string]int) {
	vars := mux.Vars(r)

	// String type query params
	search := vars["Search"]
	sort := vars["Sort"]
	order := vars["Order"]
	argsStr := map[string]string{
		"search": search,
		"order":  setOrder(sort, order),
	}

	// Integer type query params
	offset := vars["Offset"]
	limit := vars["Limit"]
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
func setOrder(sort, order string) string {
	sortValues := []string{"id", "created_at", "updated_at", "url", "username"}
	orderValues := []string{"desc", "asc"}

	if encryption.Include(sortValues, ToSnakeCase(sort)) && encryption.Include(orderValues, ToSnakeCase(order)) {
		return ToSnakeCase(sort) + " " + ToSnakeCase(order)
	}

	return "updated_at desc"
}

// Search adds where to search keywords
func setSearch(search string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if search != "" {

			// Case insensitive is different in postgres and others (mysql,sqlite)
			if viper.GetString("database.driver") == "postgres" {
				db = db.Where("url ILIKE ?", "%"+search+"%")
				db = db.Or("username ILIKE ?", "%"+search+"%")
			} else {
				db = db.Where("url LIKE ?", "%"+search+"%")
				db = db.Or("username LIKE ?", "%"+search+"%")
			}

		}
		return db
	}
}

// ToSnakeCase changes string to database table
func ToSnakeCase(str string) string {
	var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")

	return strings.ToLower(snake)
}
