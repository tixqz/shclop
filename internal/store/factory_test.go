package store

import "testing"

func TestOpenRejectsPostgresWithoutDSN(t *testing.T) {
	_, err := Open(Config{Backend: "postgres"})
	if err == nil {
		t.Fatal("expected postgres store without dsn to fail")
	}
}

func TestOpenCreatesMemoryStoreByDefault(t *testing.T) {
	opened, err := Open(Config{Backend: ""})
	if err != nil {
		t.Fatalf("open default store: %v", err)
	}
	if _, ok := opened.(*Memory); !ok {
		t.Fatalf("expected *Memory, got %T", opened)
	}
}
