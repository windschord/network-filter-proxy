package rule_test

import (
	"sync"
	"testing"

	"github.com/claudework/network-filter-proxy/internal/rule"
)

func TestStore_SetAndGet(t *testing.T) {
	s := rule.NewStore()
	entries := []rule.Entry{{Host: "example.com", Port: 443}}
	s.Set("10.0.0.1", entries)

	rs, ok := s.Get("10.0.0.1")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if len(rs.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(rs.Entries))
	}
	if rs.Entries[0].Host != "example.com" || rs.Entries[0].Port != 443 {
		t.Errorf("unexpected entry: %+v", rs.Entries[0])
	}
	if rs.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
}

func TestStore_GetNotFound(t *testing.T) {
	s := rule.NewStore()
	rs, ok := s.Get("10.0.0.1")
	if ok {
		t.Error("expected ok=false")
	}
	if rs != nil {
		t.Error("expected nil RuleSet")
	}
}

func TestStore_Delete(t *testing.T) {
	s := rule.NewStore()
	s.Set("10.0.0.1", []rule.Entry{{Host: "example.com", Port: 443}})

	ok := s.Delete("10.0.0.1")
	if !ok {
		t.Error("expected ok=true for existing key")
	}

	rs, found := s.Get("10.0.0.1")
	if found {
		t.Error("expected found=false after delete")
	}
	if rs != nil {
		t.Error("expected nil RuleSet after delete")
	}
}

func TestStore_DeleteNotFound(t *testing.T) {
	s := rule.NewStore()
	ok := s.Delete("10.0.0.1")
	if ok {
		t.Error("expected ok=false for non-existing key")
	}
}

func TestStore_DeleteAll(t *testing.T) {
	s := rule.NewStore()
	s.Set("10.0.0.1", []rule.Entry{{Host: "a.com", Port: 443}})
	s.Set("10.0.0.2", []rule.Entry{{Host: "b.com", Port: 443}})

	s.DeleteAll()
	if s.Count() != 0 {
		t.Errorf("expected 0 rules after DeleteAll, got %d", s.Count())
	}
}

func TestStore_AllIsDeepCopy(t *testing.T) {
	s := rule.NewStore()
	s.Set("10.0.0.1", []rule.Entry{{Host: "example.com", Port: 443}})

	all := s.All()
	// Modify the returned map
	all["10.0.0.1"].Entries[0].Host = "evil.com"
	delete(all, "10.0.0.1")

	// Original store should be unchanged
	rs, ok := s.Get("10.0.0.1")
	if !ok {
		t.Fatal("expected store to still have entry")
	}
	if rs.Entries[0].Host != "example.com" {
		t.Errorf("store was mutated: got %q, want %q", rs.Entries[0].Host, "example.com")
	}
}

func TestStore_Count(t *testing.T) {
	s := rule.NewStore()
	if s.Count() != 0 {
		t.Errorf("expected 0, got %d", s.Count())
	}
	s.Set("10.0.0.1", []rule.Entry{{Host: "a.com", Port: 443}})
	if s.Count() != 1 {
		t.Errorf("expected 1, got %d", s.Count())
	}
	s.Set("10.0.0.2", []rule.Entry{{Host: "b.com", Port: 443}})
	if s.Count() != 2 {
		t.Errorf("expected 2, got %d", s.Count())
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	s := rule.NewStore()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(ip string) {
			defer wg.Done()
			s.Set(ip, []rule.Entry{{Host: "example.com", Port: 443}})
		}("10.0.0." + string(rune('0'+i%10)))
		go func(ip string) {
			defer wg.Done()
			s.Get(ip)
		}("10.0.0." + string(rune('0'+i%10)))
	}
	wg.Wait()
}
