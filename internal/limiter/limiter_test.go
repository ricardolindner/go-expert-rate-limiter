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
		return 0, errors.New("erro simulado ao incrementar")
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
		return 0, errors.New("erro simulado ao obter")
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
		return errors.New("erro simulado ao definir")
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
			name:            "Deve permitir todas as requisições dentro do limite",
			key:             "ip:1.1.1.1",
			maxRequests:     5,
			blockTime:       1 * time.Minute,
			numAttempts:     5,
			expectedAllowed: 5,
			expectedError:   false,
		},
		{
			name:            "Deve bloquear requisições que excedem o limite",
			key:             "ip:2.2.2.2",
			maxRequests:     3,
			blockTime:       1 * time.Minute,
			numAttempts:     5,
			expectedAllowed: 3,
			expectedError:   false,
		},
		{
			name:            "Deve permitir todas as requisições de token dentro do limite",
			key:             "token:ABC",
			maxRequests:     10,
			blockTime:       2 * time.Minute,
			numAttempts:     10,
			expectedAllowed: 10,
			expectedError:   false,
		},
		{
			name:            "Deve bloquear requisições de token que excedem o limite",
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
				allowed, _, err := l.Allow(tt.key, tt.maxRequests, tt.blockTime)
				if (err != nil) != tt.expectedError {
					t.Fatalf("Erro inesperado: %v, esperado erro: %t", err, tt.expectedError)
				}
				if allowed {
					allowedCount++
				}
			}

			if allowedCount != tt.expectedAllowed {
				t.Errorf("Esperado %d requisições permitidas, obteve %d", tt.expectedAllowed, allowedCount)
			}

			finalCount, _ := mockStorage.Get(tt.key)
			if finalCount != tt.expectedAllowed {
				t.Errorf("Esperado %d requisições no storage, obteve %d", tt.expectedAllowed, finalCount)
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
		t.Errorf("Esperado um erro, mas não houve erro")
	}
	if allowed {
		t.Errorf("Não esperado que a requisição fosse permitida quando há erro no storage")
	}
}

func TestLimiterConcurrentRequests(t *testing.T) {
	mockStorage := NewMockStorage()
	l := limiter.NewLimiter(mockStorage)

	key := "concurrent_test_key"
	maxRequests := 10
	blockTime := 1 * time.Second

	numGoroutines := 100
	allowedCountChan := make(chan bool, numGoroutines)
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, _, err := l.Allow(key, maxRequests, blockTime)
			if err != nil {
				t.Errorf("Erro inesperado em goroutine: %v", err)
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
		t.Errorf("Esperado %d requisições permitidas em concorrência, obteve %d", maxRequests, countAllowed)
	}

	finalCount, _ := mockStorage.Get(key)
	if finalCount != maxRequests {
		t.Errorf("Esperado %d requisições permitidas no storage, obteve %d", maxRequests, finalCount)
	}
}
