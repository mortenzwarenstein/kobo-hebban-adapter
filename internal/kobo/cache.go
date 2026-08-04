package kobo

import "sync"

type BookMeta struct {
	Title  string
	Author string
}

type BookCache struct {
	mu    sync.RWMutex
	books map[string]BookMeta
}

func NewBookCache() *BookCache {
	return &BookCache{books: make(map[string]BookMeta)}
}

func (c *BookCache) Set(id string, meta BookMeta) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.books[id] = meta
}

func (c *BookCache) Get(id string) (BookMeta, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m, ok := c.books[id]
	return m, ok
}
