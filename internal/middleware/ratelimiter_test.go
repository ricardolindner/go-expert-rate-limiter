package middleware

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	config "github.com/ricardolindner/go-expert-rate-limiter/configs"
	"github.com/ricardolindner/go-expert-rate-limiter/internal/limiter"
	"github.com/ricardolindner/go-expert-rate-limiter/internal/repository"
)

var redisClient *redis.Client
var ctx context.Context

const exposedPorts = "6379/tcp"
const requestXStatusMessage = "Request %d: Expected OK status (%d), got %d"
const expectingToManyRequestsMessage = "Request %d: Expected status Too Many Requests (%d), got %d"

func TestMain(m *testing.M) {
	ctx = context.Background()

	req := tc.GenericContainerRequest{
		ContainerRequest: tc.ContainerRequest{
			Image:        "redis:6.2-alpine",
			ExposedPorts: []string{exposedPorts},
			WaitingFor:   wait.ForListeningPort(exposedPorts),
		},
		Started: true,
	}
	redisContainer, err := tc.GenericContainer(ctx, req)
	if err != nil {
		fmt.Printf("Error starting Redis container: %v\n", err)
		os.Exit(1)
	}

	ip, err := redisContainer.Host(ctx)
	if err != nil {
		fmt.Printf("Error getting IP from Redis container: %v\n", err)
		os.Exit(1)
	}
	port, err := redisContainer.MappedPort(ctx, exposedPorts)
	if err != nil {
		fmt.Printf("Error getting port from Redis container: %v\n", err)
		os.Exit(1)
	}

	redisAddr := fmt.Sprintf("%s:%s", ip, port.Port())
	fmt.Printf("Redis running on: %s\n", redisAddr)

	redisClient = redis.NewClient(&redis.Options{
		Addr: redisAddr,
		DB:   0,
	})

	err = redisClient.Ping(ctx).Err()
	if err != nil {
		fmt.Printf("Error connecting to test Redis: %v\n", err)
		redisContainer.Terminate(ctx)
		os.Exit(1)
	}

	code := m.Run()

	if err := redisContainer.Terminate(ctx); err != nil {
		fmt.Printf("Error terminating Redis container: %v\n", err)
	}

	os.Exit(code)
}

func setupTest(t *testing.T, cfg *config.Config) *http.ServeMux {

	redisClient.FlushDB(ctx)

	redisStorage := repository.NewRedisStorage(redisClient.Options().Addr, redisClient.Options().Password, redisClient.Options().DB)
	rateLimiter := limiter.NewLimiter(redisStorage)

	mux := http.NewServeMux()
	mux.Handle("/", RateLimiterMiddleware(rateLimiter, cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})))
	return mux
}

func TestRateLimiterMiddlewareIPBased(t *testing.T) {
	defaultMaxRequests := 3
	defaultBlockTime := 1 * time.Second

	cfg := &config.Config{
		DefaultMaxRequests: defaultMaxRequests,
		DefaultBlockTime:   defaultBlockTime,
	}

	mux := setupTest(t, cfg)

	ipAddr := "192.168.1.1:12345"
	expectedBlockedRequests := 0

	for i := 1; i <= defaultMaxRequests+2; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = ipAddr
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if i <= defaultMaxRequests {
			if rr.Code != http.StatusOK {
				t.Errorf(requestXStatusMessage, i, http.StatusOK, rr.Code)
			}
		} else {
			expectedBlockedRequests++
			if rr.Code != http.StatusTooManyRequests {
				t.Errorf(expectingToManyRequestsMessage, i, http.StatusTooManyRequests, rr.Code)
			}
			body, _ := io.ReadAll(rr.Body)
			if string(body) != "you have reached the maximum number of requests or actions allowed within a certain time frame\n" {
				t.Errorf("Mensagem de erro incorreta para requisição %d: %s", i, string(body))
			}
			retryAfter := rr.Header().Get("Retry-After")
			if _, err := strconv.Atoi(retryAfter); err != nil {
				t.Errorf("Header Retry-After ausente ou inválido: %s", retryAfter)
			}
		}
	}

	if expectedBlockedRequests != 2 {
		t.Errorf("Expected 2 blocked requests, got %d", expectedBlockedRequests)
	}

	t.Logf("Expecting %v for IP %s...", defaultBlockTime, ipAddr)
	time.Sleep(defaultBlockTime + 100*time.Millisecond)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = ipAddr
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("After blocking time, expected OK status, obtained %d", rr.Code)
	}
}

func TestRateLimiterMiddlewareTokenBased(t *testing.T) {
	testToken := "MY_TEST_TOKEN"
	tokenMaxRequests := 5
	tokenBlockTime := 1 * time.Second

	cfg := &config.Config{
		DefaultMaxRequests: 100,
		DefaultBlockTime:   1 * time.Minute,
		TokenLimits: map[string]config.TokenLimitConfig{
			testToken: {MaxRequests: tokenMaxRequests, BlockTime: tokenBlockTime},
		},
	}

	mux := setupTest(t, cfg)

	expectedBlockedRequests := 0
	for i := 1; i <= tokenMaxRequests+2; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("API_KEY", testToken)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if i <= tokenMaxRequests {
			if rr.Code != http.StatusOK {
				t.Errorf(requestXStatusMessage, i, http.StatusOK, rr.Code)
			}
		} else {
			expectedBlockedRequests++
			if rr.Code != http.StatusTooManyRequests {
				t.Errorf(expectingToManyRequestsMessage, i, http.StatusTooManyRequests, rr.Code)
			}
		}
	}

	if expectedBlockedRequests != 2 {
		t.Errorf("Expected 2 requests blocked by token, got %d", expectedBlockedRequests)
	}

	t.Logf("Expecting %v for token %s...", tokenBlockTime, testToken)
	time.Sleep(tokenBlockTime + 100*time.Millisecond)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("API_KEY", testToken)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("After token blocking time, expected OK status, obtained %d", rr.Code)
	}
}

func TestRateLimiterMiddlewareTokenPriority(t *testing.T) {
	testToken := "PRIORITY_TOKEN"
	tokenMaxRequests := 2
	tokenBlockTime := 1 * time.Second

	cfg := &config.Config{
		DefaultMaxRequests: 10,
		DefaultBlockTime:   1 * time.Minute,
		TokenLimits: map[string]config.TokenLimitConfig{
			testToken: {MaxRequests: tokenMaxRequests, BlockTime: tokenBlockTime},
		},
	}

	mux := setupTest(t, cfg)

	ipAddr := "192.168.1.5:12345"
	expectedBlockedRequests := 0

	for i := 1; i <= tokenMaxRequests+2; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = ipAddr
		req.Header.Set("API_KEY", testToken)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if i <= tokenMaxRequests {
			if rr.Code != http.StatusOK {
				t.Errorf(requestXStatusMessage, i, http.StatusOK, rr.Code)
			}
		} else {
			expectedBlockedRequests++
			if rr.Code != http.StatusTooManyRequests {
				t.Errorf(expectingToManyRequestsMessage, i, http.StatusTooManyRequests, rr.Code)
			}
		}
	}

	if expectedBlockedRequests != 2 {
		t.Errorf("Expected 2 requests blocked by token (priority), got %d", expectedBlockedRequests)
	}
}

func TestRateLimiterMiddlewareUnknownTokenFallbackIP(t *testing.T) {
	unknownToken := "UNKNOWN_TOKEN"
	defaultMaxRequests := 2
	defaultBlockTime := 1 * time.Second

	cfg := &config.Config{
		DefaultMaxRequests: defaultMaxRequests,
		DefaultBlockTime:   defaultBlockTime,
		TokenLimits:        map[string]config.TokenLimitConfig{},
	}

	mux := setupTest(t, cfg)

	ipAddr := "192.168.1.6:12345"
	expectedBlockedRequests := 0

	for i := 1; i <= defaultMaxRequests+2; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = ipAddr
		req.Header.Set("API_KEY", unknownToken)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if i <= defaultMaxRequests {
			if rr.Code != http.StatusOK {
				t.Errorf(requestXStatusMessage, i, http.StatusOK, rr.Code)
			}
		} else {
			expectedBlockedRequests++
			if rr.Code != http.StatusTooManyRequests {
				t.Errorf(expectingToManyRequestsMessage, i, http.StatusTooManyRequests, rr.Code)
			}
		}
	}

	if expectedBlockedRequests != 2 {
		t.Errorf("Expected 2 requests blocked by IP limit (fallback), got %d", expectedBlockedRequests)
	}
}

func TestRateLimiterMiddlewareConcurrentRequests(t *testing.T) {
	defaultMaxRequests := 10
	defaultBlockTime := 1 * time.Second

	cfg := &config.Config{
		DefaultMaxRequests: defaultMaxRequests,
		DefaultBlockTime:   defaultBlockTime,
	}

	mux := setupTest(t, cfg)
	ipAddr := "192.168.1.100:12345"
	numConcurrentRequests := 50

	var wg sync.WaitGroup
	statusCodes := make(chan int, numConcurrentRequests)

	for i := 0; i < numConcurrentRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = ipAddr
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)
			statusCodes <- rr.Code
		}()
	}

	wg.Wait()
	close(statusCodes)

	allowedCount := 0
	blockedCount := 0

	for status := range statusCodes {
		if status == http.StatusOK {
			allowedCount++
		} else if status == http.StatusTooManyRequests {
			blockedCount++
		} else {
			t.Errorf("Unexpected status: %d", status)
		}
	}

	if allowedCount != defaultMaxRequests {
		t.Errorf("Expected %d requests allowed, got %d", defaultMaxRequests, allowedCount)
	}
	if blockedCount != numConcurrentRequests-defaultMaxRequests {
		t.Errorf("Expected %d blocked requests, got %d", numConcurrentRequests-defaultMaxRequests, blockedCount)
	}
}
