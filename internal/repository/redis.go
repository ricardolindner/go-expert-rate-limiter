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
		panic(fmt.Sprintf("Não foi possível conectar ao Redis: %v", err))
	}
	fmt.Println("Conectado ao Redis com sucesso!")

	return &RedisStorage{
		client: client,
		ctx:    ctx,
	}
}

func (r *RedisStorage) Increment(key string, expiry time.Duration) (int, error) {
	pipe := r.client.Pipeline()
	val := pipe.Incr(r.ctx, key)
	pipe.Expire(r.ctx, key, expiry)
	_, err := pipe.Exec(r.ctx)
	if err != nil {
		return 0, fmt.Errorf("erro ao incrementar no Redis: %w", err)
	}
	return int(val.Val()), nil
}

func (r *RedisStorage) Get(key string) (int, error) {
	val, err := r.client.Get(r.ctx, key).Result()
	if err == redis.Nil {
		return 0, nil // Chave não encontrada
	} else if err != nil {
		return 0, fmt.Errorf("erro ao obter do Redis: %w", err)
	}
	intValue, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("erro ao converter valor do Redis para int: %w", err)
	}
	return intValue, nil
}

func (r *RedisStorage) Set(key string, value int, expiry time.Duration) error {
	return r.client.Set(r.ctx, key, value, expiry).Err()
}
