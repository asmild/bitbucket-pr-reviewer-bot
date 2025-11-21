package bitbucket

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/config"
)

func TestNewClient(t *testing.T) {
	cfg := &config.Config{
		Bitbucket: config.BitbucketConfig{
			User:  "testuser",
			Token: "testtoken",
		},
	}

	client := NewClient(cfg)

	if client == nil {
		t.Fatalf("expected non-nil client")
	}

	if client.user != "testuser" {
		t.Errorf("expected user 'testuser', got %s", client.user)
	}

	if client.token != "testtoken" {
		t.Errorf("expected token 'testtoken', got %s", client.token)
	}

	if client.baseURL != "" {
		t.Errorf("expected empty baseURL (to be set from webhook), got %s", client.baseURL)
	}

	if client.client == nil {
		t.Fatalf("expected non-nil HTTP client")
	}

	if client.emojiTracker == nil {
		t.Fatalf("expected non-nil emoji tracker")
	}

	if client.emojiTracker.current == nil {
		t.Fatalf("expected non-nil emoji tracker map")
	}
}

func TestReplyToComment_Success(t *testing.T) {
	requestReceived := false
	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true

		// Verify method
		if r.Method != "POST" {
			t.Errorf("expected POST method, got %s", r.Method)
		}

		// Verify URL structure
		expectedPath := "/rest/api/1.0/projects/PROJ/repos/test-repo/pull-requests/42/comments"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
		}

		// Verify authentication
		user, pass, ok := r.BasicAuth()
		if !ok || user != "testuser" || pass != "testtoken" {
			t.Errorf("expected valid basic auth")
		}

		// Parse payload
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": 124, "text": "Reply"}`))
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		user:    "testuser",
		token:   "testtoken",
		client:  &http.Client{},
		emojiTracker: &EmojiTracker{
			current: make(map[int]string),
			mu:      sync.Mutex{},
		},
	}

	err := client.ReplyToComment("PROJ", "test-repo", 42, 123, "This is a reply")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !requestReceived {
		t.Fatalf("expected request to be received")
	}

	if receivedPayload["text"] != "This is a reply" {
		t.Errorf("expected text 'This is a reply', got %v", receivedPayload["text"])
	}

	parent, ok := receivedPayload["parent"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected parent to be an object")
	}

	if parent["id"] != float64(123) {
		t.Errorf("expected parent ID 123, got %v", parent["id"])
	}
}

func TestReplyToComment_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		user:    "testuser",
		token:   "testtoken",
		client:  &http.Client{},
		emojiTracker: &EmojiTracker{
			current: make(map[int]string),
			mu:      sync.Mutex{},
		},
	}

	err := client.ReplyToComment("PROJ", "test-repo", 42, 123, "Reply")
	if err == nil {
		t.Fatalf("expected error for 500 response")
	}
}

func TestReplyToComment_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		user:    "testuser",
		token:   "testtoken",
		client:  &http.Client{},
		emojiTracker: &EmojiTracker{
			current: make(map[int]string),
			mu:      sync.Mutex{},
		},
	}

	// Should not error even with invalid JSON marshaling scenario
	err := client.ReplyToComment("PROJ", "test-repo", 42, 123, "Valid text")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestPostComment_Success(t *testing.T) {
	requestReceived := false
	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true

		// Verify method
		if r.Method != "POST" {
			t.Errorf("expected POST method, got %s", r.Method)
		}

		// Verify URL structure
		expectedPath := "/rest/api/1.0/projects/PROJ/repos/test-repo/pull-requests/42/comments"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
		}

		// Parse payload
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": 456, "text": "New comment"}`))
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		user:    "testuser",
		token:   "testtoken",
		client:  &http.Client{},
		emojiTracker: &EmojiTracker{
			current: make(map[int]string),
			mu:      sync.Mutex{},
		},
	}

	err := client.PostComment("PROJ", "test-repo", 42, "This is a comment")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !requestReceived {
		t.Fatalf("expected request to be received")
	}

	if receivedPayload["text"] != "This is a comment" {
		t.Errorf("expected text 'This is a comment', got %v", receivedPayload["text"])
	}

	// Verify no parent field (unlike ReplyToComment)
	if _, exists := receivedPayload["parent"]; exists {
		t.Errorf("expected no parent field for PostComment")
	}
}

func TestPostComment_EmptyText(t *testing.T) {
	requestReceived := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		user:    "testuser",
		token:   "testtoken",
		client:  &http.Client{},
		emojiTracker: &EmojiTracker{
			current: make(map[int]string),
			mu:      sync.Mutex{},
		},
	}

	// Empty text should still be sent
	err := client.PostComment("PROJ", "test-repo", 42, "")
	if err != nil {
		t.Fatalf("expected no error for empty text, got %v", err)
	}

	if !requestReceived {
		t.Fatalf("expected request to be received")
	}
}

func TestPostComment_BadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		user:    "testuser",
		token:   "testtoken",
		client:  &http.Client{},
		emojiTracker: &EmojiTracker{
			current: make(map[int]string),
			mu:      sync.Mutex{},
		},
	}

	err := client.PostComment("PROJ", "test-repo", 42, "Comment")
	if err == nil {
		t.Fatalf("expected error for 400 response")
	}
}

func TestPostComment_ContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", contentType)
		}

		accept := r.Header.Get("Accept")
		if accept != "application/json" {
			t.Errorf("expected Accept application/json, got %s", accept)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		user:    "testuser",
		token:   "testtoken",
		client:  &http.Client{},
		emojiTracker: &EmojiTracker{
			current: make(map[int]string),
			mu:      sync.Mutex{},
		},
	}

	_ = client.PostComment("PROJ", "test-repo", 42, "Comment")
}

func TestClient_ConcurrentOperations(t *testing.T) {
	callCount := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		user:    "testuser",
		token:   "testtoken",
		client:  &http.Client{},
		emojiTracker: &EmojiTracker{
			current: make(map[int]string),
			mu:      sync.Mutex{},
		},
	}

	// Test concurrent PostComment calls
	var wg sync.WaitGroup
	for i := 1; i <= 5; i++ {
		wg.Add(1)
		go func(prID int) {
			defer wg.Done()
			_ = client.PostComment("PROJ", "repo", prID, "Concurrent comment")
		}(i)
	}

	wg.Wait()

	mu.Lock()
	if callCount != 5 {
		t.Errorf("expected 5 calls, got %d", callCount)
	}
	mu.Unlock()
}

func TestPostComment_SpecialCharacters(t *testing.T) {
	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		user:    "testuser",
		token:   "testtoken",
		client:  &http.Client{},
		emojiTracker: &EmojiTracker{
			current: make(map[int]string),
			mu:      sync.Mutex{},
		},
	}

	specialText := "Test with \"quotes\" and \n newlines and 中文"
	err := client.PostComment("PROJ", "test-repo", 42, specialText)
	if err != nil {
		t.Fatalf("expected no error with special characters, got %v", err)
	}

	if receivedPayload["text"] != specialText {
		t.Errorf("expected text to be preserved with special characters")
	}
}

func TestClient_URLConstruction(t *testing.T) {
	tests := []struct {
		name          string
		operation     string
		projectKey    string
		repoSlug      string
		prID          int
		expectedPath  string
	}{
		{
			name:         "PostComment",
			operation:    "PostComment",
			projectKey:   "PROJ",
			repoSlug:     "my-repo",
			prID:         100,
			expectedPath: "/rest/api/1.0/projects/PROJ/repos/my-repo/pull-requests/100/comments",
		},
		{
			name:         "ReplyToComment",
			operation:    "ReplyToComment",
			projectKey:   "CI",
			repoSlug:     "test",
			prID:         1,
			expectedPath: "/rest/api/1.0/projects/CI/repos/test/pull-requests/1/comments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receivedPath := ""

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedPath = r.URL.Path
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{}`))
			}))
			defer server.Close()

			client := &Client{
				baseURL: server.URL,
				user:    "testuser",
				token:   "testtoken",
				client:  &http.Client{},
				emojiTracker: &EmojiTracker{
					current: make(map[int]string),
					mu:      sync.Mutex{},
				},
			}

			switch tt.operation {
			case "PostComment":
				_ = client.PostComment(tt.projectKey, tt.repoSlug, tt.prID, "test")
			case "ReplyToComment":
				_ = client.ReplyToComment(tt.projectKey, tt.repoSlug, tt.prID, 123, "test")
			}

			if receivedPath != tt.expectedPath {
				t.Errorf("expected path %s, got %s", tt.expectedPath, receivedPath)
			}
		})
	}
}

func TestEmojiTracker_Initialization(t *testing.T) {
	cfg := &config.Config{
		Bitbucket: config.BitbucketConfig{
			User:  "testuser",
			Token: "testtoken",
		},
	}

	client := NewClient(cfg)

	// Verify tracker can be used immediately
	client.emojiTracker.mu.Lock()
	client.emojiTracker.current[1] = "test"
	value, exists := client.emojiTracker.current[1]
	client.emojiTracker.mu.Unlock()

	if !exists || value != "test" {
		t.Errorf("expected tracker to work immediately after initialization")
	}
}
