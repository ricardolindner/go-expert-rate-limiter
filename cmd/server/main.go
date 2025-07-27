package main

import (
	"log"
	"net/http"
	"time"

	config "github.com/ricardolindner/go-expert-rate-limiter/configs"
	"github.com/ricardolindner/go-expert-rate-limiter/internal/limiter"
	"github.com/ricardolindner/go-expert-rate-limiter/internal/middleware"
	"github.com/ricardolindner/go-expert-rate-limiter/internal/repository"
)

func main() {
	cfg := config.LoadConfig()

	redisStorage := repository.NewRedisStorage(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)

	rateLimiter := limiter.NewLimiter(redisStorage)

	mux := http.NewServeMux()

	mux.Handle("/", middleware.RateLimiterMiddleware(rateLimiter, cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Success! Request ok."))
	})))

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("Server listening on port %s...", server.Addr)
	log.Fatal(server.ListenAndServe())
}
