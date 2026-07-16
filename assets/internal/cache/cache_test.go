package cache

import (
	"testing"
)

func TestGetMissReturnsNilFalseNil(t *testing.T) {
	c := New(t.TempDir())

	data, ok, err := c.Get("missing-key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ok {
		t.Fatal("ok = true, want false for a cache miss")
	}
	if data != nil {
		t.Fatalf("data = %v, want nil", data)
	}
}

func TestPutThenGetRoundtrips(t *testing.T) {
	c := New(t.TempDir())
	want := []byte("icon bytes")

	if err := c.Put("icon-key", want); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, ok, err := c.Get("icon-key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("ok = false, want true after Put")
	}
	if string(got) != string(want) {
		t.Fatalf("Get = %q, want %q", got, want)
	}
}

func TestPutOverwritesExistingEntry(t *testing.T) {
	c := New(t.TempDir())

	if err := c.Put("key", []byte("v1")); err != nil {
		t.Fatalf("Put v1: %v", err)
	}
	if err := c.Put("key", []byte("v2")); err != nil {
		t.Fatalf("Put v2: %v", err)
	}

	got, ok, err := c.Get("key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok || string(got) != "v2" {
		t.Fatalf("Get = (%q, %v), want (\"v2\", true)", got, ok)
	}
}

func TestDistinctKeysMapToDistinctEntries(t *testing.T) {
	c := New(t.TempDir())

	if err := c.Put("key-a", []byte("a")); err != nil {
		t.Fatalf("Put a: %v", err)
	}
	if err := c.Put("key-b", []byte("b")); err != nil {
		t.Fatalf("Put b: %v", err)
	}

	a, _, err := c.Get("key-a")
	if err != nil {
		t.Fatalf("Get a: %v", err)
	}
	b, _, err := c.Get("key-b")
	if err != nil {
		t.Fatalf("Get b: %v", err)
	}
	if string(a) != "a" || string(b) != "b" {
		t.Fatalf("a=%q b=%q, want a=\"a\" b=\"b\"", a, b)
	}
}
