package middleware

// CORS ...
// TODO: buraya el at
func CORS() {
	// return func(w http.ResponseWriter, r *http.Request) {
	// 	w.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	// 	w.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
	// 	w.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
	// 	w.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, HEAD")
	// 	if w.Request.Method == "OPTIONS" {
	// 		w.AbortWithStatus(204)
	// 		return
	// 	}
	// 	w.Next()
	// }
}
