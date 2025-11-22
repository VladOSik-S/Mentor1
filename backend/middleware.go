package main

import (
	"context"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const userIDKey contextKey = "userID"

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		isAllowed := false

		// Если Origin пустой, используем Referer
		if origin == "" {
			host := r.Host
			log.Printf("Empty origin, checking Host: %s", host)
		}

		allowedOrigins := []string{
			"https://ravishing-rebirth-production.up.railway.app",
			"https://frontend-develop-ab47.up.railway.app",
			"http://localhost:3000",
			"http://localhost:8080",
		}

		// ВАЖНО: Устанавливаем заголовки ДО проверки метода
		if slices.Contains(allowedOrigins, origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			isAllowed = true
		}

		// Устанавливаем остальные заголовки независимо от origin
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Credentials можно установить только если origin точно указан
		if isAllowed {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// Кэширование preflight на 1 час
		w.Header().Set("Access-Control-Max-Age", "3600")

		log.Printf("Origin: %s, Allowed: %v, Method: %s", origin, isAllowed, r.Method)

		// Для OPTIONS запросов возвращаем 204
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			respondError(w, http.StatusUnauthorized, "Missing authorization header")
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			respondError(w, http.StatusUnauthorized, "Invalid authorization format")
			return
		}

		claims := &jwt.RegisteredClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(getEnv("JWT_SECRET", "your-secret-key-change-in-production")), nil
		})

		if err != nil || !token.Valid {
			respondError(w, http.StatusUnauthorized, "Invalid token")
			return
		}

		userID, err := strconv.Atoi(claims.Subject)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "Invalid user ID in token")
			return
		}

		ctx := setUserID(r.Context(), userID)
		next(w, r.WithContext(ctx))
	}
}

func setUserID(ctx context.Context, userID int) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func getUserID(r *http.Request) int {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return 0
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(getEnv("JWT_SECRET", "your-secret-key-change-in-production")), nil
	})

	if err != nil || !token.Valid {
		return 0
	}

	userID, _ := strconv.Atoi(claims.Subject)
	return userID
}
