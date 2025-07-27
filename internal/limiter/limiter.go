package limiter

import (
	"log"
	"time"
)

type Limiter struct {
	storage LimiterStorage
}

func NewLimiter(storage LimiterStorage) *Limiter {
	return &Limiter{
		storage: storage,
	}
}

func (l *Limiter) Allow(key string, maxRequests int, blockTime time.Duration) (bool, time.Duration, error) {
	currentRequests, err := l.storage.Increment(key, blockTime)
	if err != nil {
		return false, 0, err
	}

	log.Printf("Key: %s, Current requests: %d, Limit: %d", key, currentRequests, maxRequests)

	if currentRequests > maxRequests {
		return false, blockTime, nil
	}

	return true, 0, nil
}
