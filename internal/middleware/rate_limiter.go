package middleware

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	config "github.com/ricardolindner/go-expert-rate-limiter/configs"
	"github.com/ricardolindner/go-expert-rate-limiter/internal/limiter"
)

func RateLimiterMiddleware(limiter *limiter.Limiter, cfg *config.Config) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var key string
			var maxRequests int
			var blockTime time.Duration

			apiKey := r.Header.Get("API_KEY")
			if apiKey != "" {
				if tokenConfig, ok := cfg.TokenLimits[apiKey]; ok {
					key = "token:" + apiKey
					maxRequests = tokenConfig.MaxRequests
					blockTime = tokenConfig.BlockTime
					log.Printf("Using per token limit: %s", apiKey)
				} else {
					log.Printf("Token '%s' not configured, using per-IP limit.", apiKey)
					key = "ip:" + getIP(r)
					maxRequests = cfg.DefaultMaxRequests
					blockTime = cfg.DefaultBlockTime
				}
			} else {
				key = "ip:" + getIP(r)
				maxRequests = cfg.DefaultMaxRequests
				blockTime = cfg.DefaultBlockTime
				log.Printf("Using IP limit: %s", getIP(r))
			}

			allowed, expiry, err := limiter.Allow(key, maxRequests, blockTime)
			if err != nil {
				log.Printf("Error in rate limiter for key %s: %v", key, err)
				http.Error(w, "Internal error in the rate limiter server", http.StatusInternalServerError)
				return
			}

			if !allowed {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(expiry.Seconds())))
				http.Error(w, "you have reached the maximum number of requests or actions allowed within a certain time frame", http.StatusTooManyRequests)
				log.Printf("Request blocked for %s. Limit exceeded.", key)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func getIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}
	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return ip
	}
	return r.RemoteAddr
}
