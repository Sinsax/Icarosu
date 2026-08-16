package meme_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"icarosu/internal/meme"
)

func TestStoreSaveAndRandom(t *testing.T) {
	dir, err := os.MkdirTemp(".", ".meme-test-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fake-image-bytes"))
	}))
	defer server.Close()

	s, err := meme.Open(dir)
	if err != nil {
		t.Fatalf("open meme store: %v", err)
	}

	path, err := s.SaveFromURL(context.Background(), server.URL+"/a.png")
	if err != nil {
		t.Fatalf("save from url: %v", err)
	}
	if path == "" {
		t.Fatal("expected saved path")
	}
	if s.Count() != 1 {
		t.Fatalf("expected count 1, got %d", s.Count())
	}

	randomPath, err := s.Random()
	if err != nil {
		t.Fatalf("random meme: %v", err)
	}
	if randomPath != path {
		t.Fatalf("unexpected random path: %s", randomPath)
	}

	data, err := s.Read(path)
	if err != nil {
		t.Fatalf("read meme: %v", err)
	}
	if string(data) != "fake-image-bytes" {
		t.Fatalf("unexpected data: %q", data)
	}
}
