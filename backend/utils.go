package main

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

func serveSPA(fsys fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		f, err := fsys.Open(strings.TrimPrefix(path, "/"))
		if err == nil {
			f.Close()
			http.FileServer(http.FS(fsys)).ServeHTTP(w, r)
			return
		}

		if !strings.HasPrefix(path, "/api/") {
			r.URL.Path = "/"
			http.FileServer(http.FS(fsys)).ServeHTTP(w, r)
			return
		}

		http.NotFound(w, r)
	}
}
