package mock

import (
	"context"
	"fmt"
	"sync"

	"github.com/fikrimohammad/go-dev-sdk/confloader/client"
)

// ConfigClient is an in-memory confloader.Client for tests.
type ConfigClient struct {
	mu    sync.Mutex
	store map[string]string
}

// NewConfigClient returns a ConfigClient seeded with folder/key -> value entries.
func NewConfigClient(store map[string]string) *ConfigClient {
	return &ConfigClient{store: store}
}

func (c *ConfigClient) Fetch(_ context.Context, folder, key string) (client.Fetched, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.store[folder+"/"+key]
	if !ok {
		return client.Fetched{}, fmt.Errorf("confloader/mock: key %s/%s not found", folder, key)
	}
	return client.Fetched{Value: v, Revision: "1"}, nil
}

func (c *ConfigClient) Close() error { return nil }
