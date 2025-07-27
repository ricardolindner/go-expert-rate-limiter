package limiter_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ricardolindner/go-expert-rate-limiter/internal/limiter"
)

type MockStorage struct {
	data          map[string]int
	mu            sync.Mutex
	simulateError bool
	expiryTime    map[string]time.Time
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		data:       make(map[string]int),
		expiryTime: make(map[string]time.Time),
	}
}

func (m *MockStorage) Increment(key string, expiry time.Duration) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.simulateError {
		return 0, errors.New("simulated increment error")
	}

	if m.expiryTime[key].Before(time.Now()) {
		m.data[key] = 0
	}

	m.data[key]++
	m.expiryTime[key] = time.Now().Add(expiry)

	return m.data[key], nil
}

func (m *MockStorage) Get(key string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.simulateError {
		return 0, errors.New("simulated get error")
	}

	if m.expiryTime[key].Before(time.Now()) {
		return 0, nil
	}
	return m.data[key], nil
}

func (m *MockStorage) Set(key string, value int, expiry time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.simulateError {
		return errors.New("simulated set error")
	}
	m.data[key] = value
	m.expiryTime[key] = time.Now().Add(expiry)
	return nil
}

func TestLimiterAllow(t *testing.T) {
	tests := []struct {
		name            string
		key             string
		maxRequests     int
		blockTime       time.Duration
		numAttempts     int
		expectedAllowed int
		expectedError   bool
	}{
		{
			name:            "Must allow all requests within the limit",
			key:             "ip:1.1.1.1",
			maxRequests:     5,
			blockTime:       1 * time.Minute,
			numAttempts:     5,
			expectedAllowed: 5,
			expectedError:   false,
		},
		{
			name:            "Should block requests that exceed the limit",
			key:             "ip:2.2.2.2",
			maxRequests:     3,
			blockTime:       1 * time.Minute,
			numAttempts:     5,
			expectedAllowed: 3,
			expectedError:   false,
		},
		{
			name:            "Must allow all token requests within the limit",
			key:             "token:ABC",
			maxRequests:     10,
			blockTime:       2 * time.Minute,
			numAttempts:     10,
			expectedAllowed: 10,
			expectedError:   false,
		},
		{
			name:            "Should block token requests that exceed the limit",
			key:             "token:XYZ",
			maxRequests:     2,
			blockTime:       3 * time.Minute,
			numAttempts:     4,
			expectedAllowed: 2,
			expectedError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := NewMockStorage()
			l := limiter.NewLimiter(mockStorage)

			allowedCount := 0
			for i := 0; i < tt.numAttempts; i++ {
				// CORREÇÃO AQUI: Capture o terceiro valor ou descarte-o
				allowed, _, err := l.Allow(tt.key, tt.maxRequests, tt.blockTime)
				if (err != nil) != tt.expectedError {
					t.Fatalf("Unespected error:: %v, expecting: %t", err, tt.expectedError)
				}
				if allowed {
					allowedCount++
				}
			}

			if allowedCount != tt.expectedAllowed {
				t.Errorf("Expected %d requests allowed, got %d", tt.expectedAllowed, allowedCount)
			}

			// CORREÇÃO AQUI: finalCount no storage deve ser o total de tentativas
			finalCount, _ := mockStorage.Get(tt.key)
			if finalCount != tt.numAttempts { // Mude para tt.numAttempts
				t.Errorf("Expected %d requests in storage, got %d", tt.numAttempts, finalCount)
			}
		})
	}
}

func TestLimiterAllowErrorFromStorage(t *testing.T) {
	mockStorage := NewMockStorage()
	mockStorage.simulateError = true
	l := limiter.NewLimiter(mockStorage)

	allowed, _, err := l.Allow("key", 1, 1*time.Minute)
	if err == nil {
		t.Errorf("Expected an error, but there was no error")
	}
	if allowed {
		t.Errorf("Not expected that the request would be allowed when there is an error in the storage")
	}
}

func TestLimiterConcurrentRequests(t *testing.T) {
	mockStorage := NewMockStorage()
	l := limiter.NewLimiter(mockStorage)

	key := "concurrent_test_key"
	maxRequests := 10
	blockTime := 1 * time.Second // Tempo de bloqueio curto para o teste

	numGoroutines := 100 // Tentar muitas requisições simultaneamente
	allowedCountChan := make(chan bool, numGoroutines)
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Captura os 3 valores, descartando o do meio com _
			allowed, _, err := l.Allow(key, maxRequests, blockTime)
			if err != nil {
				t.Errorf("Unexpected error in goroutine: %v", err)
				return
			}
			allowedCountChan <- allowed
		}()
	}

	wg.Wait()
	close(allowedCountChan)

	countAllowed := 0
	for allowed := range allowedCountChan {
		if allowed {
			countAllowed++
		}
	}

	if countAllowed != maxRequests {
		t.Errorf("Expected %d requests allowed in concurrency, got %d", maxRequests, countAllowed)
	}

	finalCount, _ := mockStorage.Get(key)
	if finalCount != numGoroutines {
		t.Errorf("Expected %d requests in storage (total attempts), got %d", numGoroutines, finalCount)
	}
}
