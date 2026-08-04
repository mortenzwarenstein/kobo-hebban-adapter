package kobo

import (
	"sort"
	"strings"
	"testing"
)

func TestUserStore_LookupUnknownToken(t *testing.T) {
	us := NewUserStore(map[string]UserEntry{})
	hc, bc, ok := us.Lookup("nope")
	if ok || hc != nil || bc != nil {
		t.Fatalf("got hc=%v bc=%v ok=%v, want nil, nil, false", hc, bc, ok)
	}
}

func TestUserStore_LookupKnownToken(t *testing.T) {
	us := NewUserStore(map[string]UserEntry{
		"tok1": {Name: "Alice", HebbanToken: "h1"},
	})
	hc, bc, ok := us.Lookup("tok1")
	if !ok {
		t.Fatal("expected ok=true for a known token")
	}
	if hc == nil {
		t.Error("expected a non-nil hebban.Client")
	}
	if bc == nil {
		t.Error("expected a non-nil BookCache")
	}
}

func TestUserStore_LookupReturnsSameCacheAcrossCalls(t *testing.T) {
	us := NewUserStore(map[string]UserEntry{
		"tok1": {Name: "Alice", HebbanToken: "h1"},
	})
	_, bc1, _ := us.Lookup("tok1")
	bc1.Set("book1", BookMeta{Title: "The Hobbit"})

	_, bc2, _ := us.Lookup("tok1")
	got, ok := bc2.Get("book1")
	if !ok || got.Title != "The Hobbit" {
		t.Fatalf("expected repeated Lookup calls to share the same BookCache, got ok=%v got=%+v", ok, got)
	}
}

func TestUserStore_CacheIsolationBetweenUsers(t *testing.T) {
	us := NewUserStore(map[string]UserEntry{
		"tok1": {Name: "Alice", HebbanToken: "h1"},
		"tok2": {Name: "Bob", HebbanToken: "h2"},
	})
	_, bc1, _ := us.Lookup("tok1")
	_, bc2, _ := us.Lookup("tok2")

	bc1.Set("book1", BookMeta{Title: "Alice's book"})

	if _, ok := bc2.Get("book1"); ok {
		t.Fatal("expected Bob's cache to be isolated from Alice's cache")
	}
}

func TestUserStore_Users(t *testing.T) {
	us := NewUserStore(map[string]UserEntry{
		"tok1": {Name: "Alice", HebbanToken: "h1"},
		"tok2": {Name: "Bob", HebbanToken: "h2"},
	})

	users := us.Users()
	sort.Slice(users, func(i, j int) bool { return users[i].Token < users[j].Token })

	want := []User{{Token: "tok1", Name: "Alice"}, {Token: "tok2", Name: "Bob"}}
	if len(users) != len(want) {
		t.Fatalf("got %d users, want %d", len(users), len(want))
	}
	for i := range want {
		if users[i] != want[i] {
			t.Errorf("users[%d] = %+v, want %+v", i, users[i], want[i])
		}
	}
}

func TestUserStore_LookupPropagatesEmptyHebbanToken(t *testing.T) {
	us := NewUserStore(map[string]UserEntry{
		"tok1": {Name: "Alice", HebbanToken: ""},
	})
	hc, _, ok := us.Lookup("tok1")
	if !ok {
		t.Fatal("expected ok=true")
	}
	err := hc.UpdateReadingStatus("Some Book", "Some Author", "reading")
	if err == nil || !strings.Contains(err.Error(), "no Hebban token configured") {
		t.Fatalf("got err=%v, want an error confirming the empty token flowed through to hebban.Client", err)
	}
}
