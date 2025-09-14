package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/time/rate"
	_ "modernc.org/sqlite" // Changed this line
)

type Share struct {
	Code      string    `json:"code"`
	Data      string    `json:"data"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type ShareRequest struct {
	Code           string `json:"code"`
	Data           string `json:"data"`
	ExpiresMinutes int    `json:"expires_minutes"`
}

var db *sql.DB

var (
	rateLimiters     = make(map[string]*rate.Limiter)
	rateLimiterMutex = &sync.RWMutex{}
)

func initDB() {
	var err error
	db, err = sql.Open("sqlite", "./relay.db")
	if err != nil {
		log.Fatal(err)
	}

	createTable := `
	CREATE TABLE IF NOT EXISTS shares (
		code TEXT PRIMARY KEY,
		data TEXT NOT NULL,
		expires_at DATETIME NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_expires_at ON shares(expires_at);
	`
	_, err = db.Exec(createTable)
	if err != nil {
		log.Fatal(err)
	}
}

// func cleanupExpired() {
// 	for {
// 		_, err := db.Exec("DELETE FROM shares WHERE expires_at < datetime('now')")
// 		if err != nil {
// 			log.Printf("Cleanup error: %v", err)
// 		}
// 		time.Sleep(1 * time.Minute)
// 	}
// }

func cleanupExpired() {
	ticker := time.NewTicker(30 * time.Second) // More frequent cleanup
	defer ticker.Stop()

	for range ticker.C {
		result, err := db.Exec("DELETE FROM shares WHERE expires_at < datetime('now')")
		if err != nil {
			log.Printf("Cleanup error: %v", err)
			continue
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			log.Printf("Failed to get rows affected: %v", err)
			continue
		}

		if rowsAffected > 0 {
			log.Printf("Cleaned up %d expired shares", rowsAffected)
		}
	}
}

func getRateLimiter(ip string) *rate.Limiter {
	rateLimiterMutex.RLock()
	limiter, exists := rateLimiters[ip]
	rateLimiterMutex.RUnlock()

	if !exists {
		rateLimiterMutex.Lock()
		limiter = rate.NewLimiter(rate.Every(time.Minute), 10) // 10 requests per minute
		rateLimiters[ip] = limiter
		rateLimiterMutex.Unlock()
	}
	return limiter
}

func rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		limiter := getRateLimiter(ip)

		if !limiter.Allow() {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func checkCodeHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	code := vars["code"]

	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM shares WHERE code = ?)", code).Scan(&exists)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if exists {
		http.Error(w, "Code in use", http.StatusConflict)
	} else {
		w.WriteHeader(http.StatusNotFound) // Available
	}
}

func shareHandler(w http.ResponseWriter, r *http.Request) {
	var req ShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Code == "" || req.Data == "" {
		http.Error(w, "Missing code or data", http.StatusBadRequest)
		return
	}

	var expiresMinutes = 10

	expiresAt := time.Now().Add(time.Duration(expiresMinutes) * time.Minute)

	_, err := db.Exec(
		"INSERT OR REPLACE INTO shares (code, data, expires_at) VALUES (?, ?, ?)",
		req.Code, req.Data, expiresAt,
	)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status":     "created",
		"expires_at": expiresAt.Format(time.RFC3339),
	})
}

func receiveHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	code := vars["code"]

	var share Share
	err := db.QueryRow(
		"SELECT code, data, expires_at FROM shares WHERE code = ? AND expires_at > datetime('now')",
		code,
	).Scan(&share.Code, &share.Data, &share.ExpiresAt)

	if err == sql.ErrNoRows {
		http.Error(w, "Code not found or expired", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"data": share.Data,
	})
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func main() {
	initDB()
	defer db.Close()

	// Start cleanup goroutine
	go cleanupExpired()

	r := mux.NewRouter()
	r.Use(rateLimitMiddleware)
	r.HandleFunc("/health", healthHandler).Methods("GET")
	r.HandleFunc("/check/{code}", checkCodeHandler).Methods("GET")
	r.HandleFunc("/share", shareHandler).Methods("POST")
	r.HandleFunc("/receive/{code}", receiveHandler).Methods("GET")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("🚀 Relay server starting on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
