package kobo

import (
	"kobo-hebban-adapter/internal/hebban"
	"sync"
)

type UserEntry struct {
	Name        string `json:"name"`
	HebbanToken string `json:"hebbanToken"`
}

type User struct {
	Token string
	Name  string
}

type UserStore struct {
	mu     sync.RWMutex
	users  map[string]UserEntry
	caches map[string]*BookCache
}

func NewUserStore(users map[string]UserEntry) *UserStore {
	caches := make(map[string]*BookCache, len(users))
	for token := range users {
		caches[token] = NewBookCache()
	}
	return &UserStore{users: users, caches: caches}
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
