package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOffset(t *testing.T) {
	assert.Equal(t, 3, Offset("3"), "they should be equal")
}

/* func TestGetHandlerSingleValue(t *testing.T) {
	db := inc.InitDB()
	defer db.Close()

	router := gin.Default()
	router.GET("/:id", controller.GetHandler)

	w := httptest.NewRecorder()
	reqEmpty, _ := http.NewRequest("GET", "/", nil)
	router.ServeHTTP(w, reqEmpty)
	assert.Equal(t, 404, w.Code)

	w = httptest.NewRecorder()
	reqInt, _ := http.NewRequest("GET", "/424", nil)
	router.ServeHTTP(w, reqInt)
	assert.Equal(t, 200, w.Code)

	w = httptest.NewRecorder()
	reqStr, _ := http.NewRequest("GET", "/asd", nil)
	router.ServeHTTP(w, reqStr)
	assert.Equal(t, 404, w.Code)

	// assert.Equal(t, "pong", w.Body.String())
}

func TestGetHandlerMultiValue(t *testing.T) {
	db := inc.InitDB()
	defer db.Close()

	router := gin.Default()
	router.GET("/:id/:count", controller.GetHandler)

	w := httptest.NewRecorder()
	reqPing, _ := http.NewRequest("GET", "/erhan/download", nil)
	router.ServeHTTP(w, reqPing)
	assert.Equal(t, 200, w.Code)
}
*/
