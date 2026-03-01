package common

import (
	"encoding/json"
	"net/http"
)

// HandlerFunc — alias para http.HandlerFunc.
// Se trocar de framework, muda só aqui + os helpers abaixo.
type HandlerFunc = http.HandlerFunc

func PathParam(r *http.Request, name string) string {
	return r.PathValue(name)
}

func QueryParam(r *http.Request, name string) string {
	return r.URL.Query().Get(name)
}

func BindJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(dst)
}

func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func ErrorJSON(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, map[string]string{"error": msg})
}
