package middleware

import "net/http"

// CORS allows the deployed React frontend (a different origin from
// the API) to call this backend from the browser. Wide open (*)
// here deliberately — this is a public read-mostly media API with
// no cookies/session auth, so there's no cross-origin credential
// leak risk that a stricter allow-list would be protecting against.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}