package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	log.Printf("Login attempt - TelegramID: %s, Name: %s", req.TelegramID, req.Name)

	var userID int
	var role string
	var isGuest bool

	// Проверяем, есть ли пользователь в БД
	err := db.QueryRow(`
        SELECT id, role FROM users WHERE telegram_id = $1
    `, req.TelegramID).Scan(&userID, &role)

	if err == sql.ErrNoRows {
		// Создаем гостевого пользователя
		err = db.QueryRow(`
            INSERT INTO users (telegram_id, name, status, role)
            VALUES ($1, $2, 'Guest', 'guest')
            RETURNING id
        `, req.TelegramID, req.Name).Scan(&userID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to create guest user")
			return
		}

		role = "guest"
		isGuest = true

		// Назначаем дефолтный спринт гостю
		_, _ = db.Exec(`
            INSERT INTO user_sprints (user_id, sprint_id, status, started_at)
            SELECT $1, id, 'active', NOW()
            FROM sprints 
            WHERE id = 1
            ON CONFLICT DO NOTHING
        `, userID)
	}

	// Генерируем JWT токен
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   strconv.Itoa(userID),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	})

	tokenString, err := token.SignedString([]byte(getEnv("JWT_SECRET", "your-secret-key-change-in-production")))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Token generation failed")
		return
	}

	// Получаем полные данные пользователя
	var user User
	db.QueryRow(`
        SELECT id, telegram_id, name, status, role, created_at
        FROM users WHERE id = $1
    `, userID).Scan(&user.ID, &user.TelegramID, &user.Name, &user.Status, &user.Role, &user.CreatedAt)

	respondJSON(w, http.StatusOK, AuthResponse{
		Token:   tokenString,
		User:    user,
		IsGuest: isGuest,
	})
}

func handleTestAuth(w http.ResponseWriter, r *http.Request) {
	// ТОЛЬКО ДЛЯ РАЗРАБОТКИ!
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   "1", // ID тестового пользователя
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
	})

	tokenString, _ := token.SignedString([]byte(getEnv("JWT_SECRET", "your-secret-key-change-in-production")))
	respondJSON(w, http.StatusOK, map[string]string{"token": tokenString})
}

func handleAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var authReq struct {
		InitData string `json:"initData"` // Данные от Telegram.WebApp.initData
	}

	if err := json.NewDecoder(r.Body).Decode(&authReq); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Здесь нужно валидировать initData от Telegram
	// Парсим telegramId из initData
	telegramID := parseTelegramID(authReq.InitData)

	// Получаем или создаем пользователя
	var userID int
	err := db.QueryRow(`
        INSERT INTO users (telegramid, name, status)
        VALUES ($1, $2, $3)
        ON CONFLICT (telegramid) DO UPDATE SET name = $2
        RETURNING id
    `, telegramID, "User", "Junior Developer").Scan(&userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	// Генерируем JWT токен
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   strconv.Itoa(userID),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	})

	tokenString, err := token.SignedString([]byte(getEnv("JWT_SECRET", "your-secret-key-change-in-production")))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Token generation failed")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"token":  tokenString,
		"userId": userID,
	})
}

func parseTelegramID(initData string) string {
	// Упрощенный парсинг, в реальности нужно валидировать подпись
	params, _ := url.ParseQuery(initData)
	if user := params.Get("user"); user != "" {
		var userData struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal([]byte(user), &userData)
		return strconv.FormatInt(userData.ID, 10)
	}
	return "123456789" // Для тестирования
}
