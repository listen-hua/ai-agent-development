package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type SlotLocker interface {
	Lock(context.Context, string, time.Duration) (func(), bool, error)
}

type RedisSlotLocker struct {
	client *redis.Client
}

func NewRedisSlotLocker(addr string) *RedisSlotLocker {
	return &RedisSlotLocker{client: redis.NewClient(&redis.Options{Addr: addr})}
}

func (l *RedisSlotLocker) Lock(ctx context.Context, key string, ttl time.Duration) (func(), bool, error) {
	tokenBytes := make([]byte, 16)
	_, _ = rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)
	ok, err := l.client.SetNX(ctx, "meeting-slot:"+key, token, ttl).Result()
	if err != nil || !ok {
		return func() {}, ok, err
	}
	return func() {
		const script = `if redis.call("get",KEYS[1]) == ARGV[1] then return redis.call("del",KEYS[1]) else return 0 end`
		_, _ = l.client.Eval(context.Background(), script, []string{"meeting-slot:" + key}, token).Result()
	}, true, nil
}

func (l *RedisSlotLocker) Close() error { return l.client.Close() }

type localSlotLocker struct {
	mu   sync.Mutex
	keys map[string]bool
}

func NewLocalSlotLocker() SlotLocker { return &localSlotLocker{keys: map[string]bool{}} }

func (l *localSlotLocker) Lock(_ context.Context, key string, _ time.Duration) (func(), bool, error) {
	l.mu.Lock()
	if l.keys[key] {
		l.mu.Unlock()
		return func() {}, false, nil
	}
	l.keys[key] = true
	l.mu.Unlock()
	return func() {
		l.mu.Lock()
		delete(l.keys, key)
		l.mu.Unlock()
	}, true, nil
}
