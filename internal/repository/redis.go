package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

type RedisStorage struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisStorage(addr, password string, db int) *RedisStorage {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	ctx := context.Background()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		panic(fmt.Sprintf("Unable to connect to Redis: %v", err))
	}
	fmt.Println("Connected to Redis successfully!")

	return &RedisStorage{
		client: client,
		ctx:    ctx,
	}
}

func (r *RedisStorage) Increment(key string, expiry time.Duration) (int, error) {
	val, err := r.client.Incr(r.ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("error when incrementing in Redis: %w", err)
	}

	if val == 1 {
		// Só define o tempo de expiração se a chave foi criada agora
		err := r.client.Expire(r.ctx, key, expiry).Err()
		if err != nil {
			return 0, fmt.Errorf("error setting expiration in Redis: %w", err)
		}
	}

	return int(val), nil
}

func (r *RedisStorage) Get(key string) (int, error) {
	val, err := r.client.Get(r.ctx, key).Result()
	if err == redis.Nil {
		return 0, nil // Chave não encontrada
	} else if err != nil {
		return 0, fmt.Errorf("error getting from Redis: %w", err)
	}
	intValue, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("error converting Redis value to int: %w", err)
	}
	return intValue, nil
}

func (r *RedisStorage) Set(key string, value int, expiry time.Duration) error {
	return r.client.Set(r.ctx, key, value, expiry).Err()
}
