package kobo

import (
	"strconv"
	"sync"
	"testing"
)

func TestBookCache_SetGet(t *testing.T) {
	c := NewBookCache()
	c.Set("id1", BookMeta{Title: "The Hobbit", Author: "Tolkien"})

	got, ok := c.Get("id1")
	if !ok {
		t.Fatal("expected ok=true for a key that was set")
	}
	want := BookMeta{Title: "The Hobbit", Author: "Tolkien"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestBookCache_MissingKey(t *testing.T) {
	c := NewBookCache()
	got, ok := c.Get("nope")
	if ok {
		t.Fatal("expected ok=false for a missing key")
	}
	if got != (BookMeta{}) {
		t.Errorf("got %+v, want zero value", got)
	}
}

func TestBookCache_Overwrite(t *testing.T) {
	c := NewBookCache()
	c.Set("id1", BookMeta{Title: "First", Author: "A"})
	c.Set("id1", BookMeta{Title: "Second", Author: "B"})

	got, ok := c.Get("id1")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if want := (BookMeta{Title: "Second", Author: "B"}); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestBookCache_ConcurrentAccess(t *testing.T) {
	c := NewBookCache()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		id := strconv.Itoa(i)
		go func() {
			defer wg.Done()
			c.Set(id, BookMeta{Title: "Title " + id})
		}()
		go func() {
			defer wg.Done()
			c.Get(id)
		}()
	}
	wg.Wait()

	for i := 0; i < 100; i++ {
		id := strconv.Itoa(i)
		meta, ok := c.Get(id)
		if !ok {
			t.Fatalf("expected id %q to be present after concurrent writes", id)
		}
		if meta.Title != "Title "+id {
			t.Errorf("id %q: got title %q, want %q", id, meta.Title, "Title "+id)
		}
	}
}
