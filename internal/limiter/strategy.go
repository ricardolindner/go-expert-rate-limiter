package limiter

import "time"

type LimiterStorage interface {
	Increment(key string, expiry time.Duration) (int, error)
	Get(key string) (int, error)
	Set(key string, value int, expiry time.Duration) error
}
