package redis

import (
	"context"
	"sync"
	"time"

	commonredis "github.com/fikrimohammad/go-dev-sdk/redis"
	"github.com/redis/go-redis/v9"
)

type mockRedisClient struct {
	mu     sync.Mutex
	data   map[string]string
	getErr error
	setErr error
}

func newMockRedisClient() *mockRedisClient {
	return &mockRedisClient{data: make(map[string]string)}
}

func (m *mockRedisClient) Get(_ context.Context, key string) *redis.StringCmd {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getErr != nil {
		return redis.NewStringResult("", m.getErr)
	}
	val, ok := m.data[key]
	if !ok {
		return redis.NewStringResult("", redis.Nil)
	}
	return redis.NewStringResult(val, nil)
}

func (m *mockRedisClient) Set(_ context.Context, key string, _ interface{}, _ time.Duration) *redis.StatusCmd {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setErr != nil {
		return redis.NewStatusResult("", m.setErr)
	}
	m.data[key] = "1"
	return redis.NewStatusResult("OK", nil)
}

func (m *mockRedisClient) SetNX(_ context.Context, key string, _ interface{}, _ time.Duration) *redis.BoolCmd {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setErr != nil {
		return redis.NewBoolResult(false, m.setErr)
	}
	if _, ok := m.data[key]; ok {
		return redis.NewBoolResult(false, nil)
	}
	m.data[key] = "1"
	return redis.NewBoolResult(true, nil)
}

func (m *mockRedisClient) Del(_ context.Context, keys ...string) *redis.IntCmd {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range keys {
		delete(m.data, key)
	}
	return redis.NewIntResult(1, nil)
}

func (m *mockRedisClient) Ping(context.Context) *redis.StatusCmd {
	return redis.NewStatusResult("PONG", nil)
}

func (m *mockRedisClient) Close() error { return nil }

func (m *mockRedisClient) Pipeline() commonredis.Pipeline {
	return &mockRedisPipeline{m: m}
}

func (m *mockRedisClient) set(key, val string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = val
}

// mockRedisPipeline queues commands and resolves them against the shared mock
// client when Exec is called, mirroring go-redis pipeline semantics.
type mockRedisPipeline struct {
	m   *mockRedisClient
	ops []func()
}

func (p *mockRedisPipeline) Get(_ context.Context, key string) *redis.StringCmd {
	cmd := redis.NewStringCmd(context.Background(), "get", key)
	p.ops = append(p.ops, func() {
		p.m.mu.Lock()
		defer p.m.mu.Unlock()
		if p.m.getErr != nil {
			cmd.SetErr(p.m.getErr)
			return
		}
		val, ok := p.m.data[key]
		if !ok {
			cmd.SetErr(redis.Nil)
			return
		}
		cmd.SetVal(val)
	})
	return cmd
}

func (p *mockRedisPipeline) Set(_ context.Context, key string, _ interface{}, _ time.Duration) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(context.Background(), "set", key)
	p.ops = append(p.ops, func() {
		p.m.mu.Lock()
		defer p.m.mu.Unlock()
		if p.m.setErr != nil {
			cmd.SetErr(p.m.setErr)
			return
		}
		p.m.data[key] = "1"
		cmd.SetVal("OK")
	})
	return cmd
}

func (p *mockRedisPipeline) SetNX(_ context.Context, key string, _ interface{}, _ time.Duration) *redis.BoolCmd {
	cmd := redis.NewBoolCmd(context.Background(), "setnx", key)
	p.ops = append(p.ops, func() {
		p.m.mu.Lock()
		defer p.m.mu.Unlock()
		if p.m.setErr != nil {
			cmd.SetErr(p.m.setErr)
			return
		}
		if _, ok := p.m.data[key]; ok {
			cmd.SetVal(false)
			return
		}
		p.m.data[key] = "1"
		cmd.SetVal(true)
	})
	return cmd
}

func (p *mockRedisPipeline) Del(_ context.Context, keys ...string) *redis.IntCmd {
	cmd := redis.NewIntCmd(context.Background(), "del", keys)
	p.ops = append(p.ops, func() {
		p.m.mu.Lock()
		defer p.m.mu.Unlock()
		for _, key := range keys {
			delete(p.m.data, key)
		}
		cmd.SetVal(int64(len(keys)))
	})
	return cmd
}

func (p *mockRedisPipeline) Ping(context.Context) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(context.Background(), "ping")
	p.ops = append(p.ops, func() { cmd.SetVal("PONG") })
	return cmd
}

func (p *mockRedisPipeline) Exec(context.Context) error {
	for _, op := range p.ops {
		op()
	}
	return nil
}
