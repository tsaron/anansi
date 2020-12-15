package responses

import "net/http"

// Write a JSON response body and set the content type of the response
func Write(w http.ResponseWriter, code int, data []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	w.WriteHeader(code)
	_, err := w.Write(data)
	if err != nil {
		panic(err)
	}
}
