package kobo

import (
	"encoding/json"
	"fmt"
	"kobo-hebban-adapter/hebban"
	"os"
	"sync"
)

type userEntry struct {
	Name        string `json:"name"`
	HebbanToken string `json:"hebbanToken"`
}

type User struct {
	Token string
	Name  string
}

type UserStore struct {
	mu     sync.RWMutex
	users  map[string]userEntry
	caches map[string]*BookCache
}

func LoadUserStore(path string) (*UserStore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read users config: %w", err)
	}

	var cfg map[string]userEntry
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse users config: %w", err)
	}

	caches := make(map[string]*BookCache, len(cfg))
	for token := range cfg {
		caches[token] = NewBookCache()
	}

	return &UserStore{users: cfg, caches: caches}, nil
}

func (us *UserStore) Users() []User {
	us.mu.RLock()
	defer us.mu.RUnlock()
	out := make([]User, 0, len(us.users))
	for token, entry := range us.users {
		out = append(out, User{Token: token, Name: entry.Name})
	}
	return out
}

func (us *UserStore) Lookup(token string) (*hebban.Client, *BookCache, bool) {
	us.mu.RLock()
	defer us.mu.RUnlock()
	entry, ok := us.users[token]
	if !ok {
		return nil, nil, false
	}
	return hebban.NewClient(entry.HebbanToken), us.caches[token], true
}
