package bitbucket

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/asmild/bitbucket-pr-reviewer-bot/pkg/models"
)

func TestAddCommentReaction_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT method, got %s", r.Method)
		}

		// Verify emoji is in URL
		if !bytes.Contains([]byte(r.URL.Path), []byte("thumbsup")) {
			t.Errorf("expected thumbsup in URL path")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
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

	err := client.AddCommentReaction("PROJ", "repo", 1, 123, "+1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddCommentReaction_UnknownEmoji(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Unknown emoji should be passed as-is
		if !bytes.Contains([]byte(r.URL.Path), []byte("custom_emoji")) {
			t.Errorf("expected custom_emoji in URL path")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
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

	err := client.AddCommentReaction("PROJ", "repo", 1, 123, "custom_emoji")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddCommentReaction_EmojiMapping(t *testing.T) {
	tests := []struct {
		inputEmoji      string
		expectedEmojiURL string
	}{
		{"eyes", "eyes"},
		{"thinking_face", "thinking_face"},
		{"+1", "thumbsup"},
		{"x", "x"},
		{"-1", "thumbsdown"},
		{"tada", "tada"},
	}

	for _, tt := range tests {
		t.Run(tt.inputEmoji, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !bytes.Contains([]byte(r.URL.Path), []byte(tt.expectedEmojiURL)) {
					t.Errorf("expected %s in URL path, got %s", tt.expectedEmojiURL, r.URL.Path)
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("{}"))
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

			err := client.AddCommentReaction("PROJ", "repo", 1, 123, tt.inputEmoji)
			if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestRemoveCommentReaction_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE method, got %s", r.Method)
		}

		if !bytes.Contains([]byte(r.URL.Path), []byte("thumbsup")) {
			t.Errorf("expected thumbsup in URL path")
		}

		w.WriteHeader(http.StatusNoContent)
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

	err := client.RemoveCommentReaction("PROJ", "repo", 1, 123, "+1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRemoveCommentReaction_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
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

	// Should not error on 404 (emoji already removed)
	err := client.RemoveCommentReaction("PROJ", "repo", 1, 123, "+1")
	if err != nil {
		t.Fatalf("expected no error on 404, got %v", err)
	}
}

func TestRemoveCommentReaction_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
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

	err := client.RemoveCommentReaction("PROJ", "repo", 1, 123, "+1")
	if err == nil {
		t.Fatalf("expected error for 500 response")
	}
}

func TestReplaceReaction_NoCommentID(t *testing.T) {
	client := &Client{
		emojiTracker: &EmojiTracker{
			current: make(map[int]string),
			mu:      sync.Mutex{},
		},
	}

	// Should return early if CommentID is 0
	prData := &models.PRData{
		CommentID: 0,
	}

	client.ReplaceReaction(prData, "thumbsup")
	// No error should occur, function should return early
}

func TestReplaceReaction_FirstEmoji(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			if bytes.Contains([]byte(r.URL.Path), []byte("eyes")) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("{}"))
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
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

	prData := &models.PRData{
		ProjectKey: "PROJ",
		Repository: "repo",
		PRID:       1,
		CommentID:  123,
	}

	// Add first emoji (no previous emoji to remove)
	client.ReplaceReaction(prData, "eyes")

	// Verify tracker was updated
	client.emojiTracker.mu.Lock()
	current, exists := client.emojiTracker.current[123]
	client.emojiTracker.mu.Unlock()

	if !exists || current != "eyes" {
		t.Fatalf("expected tracker to contain eyes emoji")
	}
}

func TestReplaceReaction_ReplaceWithNewEmoji(t *testing.T) {
	callCount := 0
	methodOrder := []string{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		methodOrder = append(methodOrder, r.Method)

		if r.Method == "DELETE" && bytes.Contains([]byte(r.URL.Path), []byte("eyes")) {
			// Remove previous emoji
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.Method == "PUT" && bytes.Contains([]byte(r.URL.Path), []byte("thumbsup")) {
			// Add new emoji
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{}"))
			return
		}

		w.WriteHeader(http.StatusNotFound)
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

	prData := &models.PRData{
		ProjectKey: "PROJ",
		Repository: "repo",
		PRID:       1,
		CommentID:  123,
	}

	// Set initial emoji
	client.emojiTracker.current[123] = "eyes"

	// Replace with new emoji
	client.ReplaceReaction(prData, "+1")

	// Verify tracker was updated
	client.emojiTracker.mu.Lock()
	current, exists := client.emojiTracker.current[123]
	client.emojiTracker.mu.Unlock()

	if !exists || current != "+1" {
		t.Fatalf("expected tracker to contain +1 emoji, got %s", current)
	}

	// Verify DELETE was called before PUT
	if len(methodOrder) >= 2 {
		if methodOrder[0] != "DELETE" {
			t.Errorf("expected DELETE before PUT, got %s", methodOrder[0])
		}
		if methodOrder[1] != "PUT" {
			t.Errorf("expected PUT after DELETE, got %s", methodOrder[1])
		}
	}
}

func TestReplaceReaction_ConcurrentAccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
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

	// Test concurrent emoji updates for different comments
	var wg sync.WaitGroup
	for i := 1; i <= 5; i++ {
		wg.Add(1)
		go func(commentID int, emoji string) {
			defer wg.Done()
			prData := &models.PRData{
				ProjectKey: "PROJ",
				Repository: "repo",
				PRID:       1,
				CommentID:  commentID,
			}
			client.ReplaceReaction(prData, emoji)
		}(i, "eyes")
	}

	wg.Wait()

	// Verify all emojis were tracked
	client.emojiTracker.mu.Lock()
	if len(client.emojiTracker.current) != 5 {
		t.Fatalf("expected 5 tracked emojis, got %d", len(client.emojiTracker.current))
	}
	client.emojiTracker.mu.Unlock()
}

func TestEmojiMapCompleteness(t *testing.T) {
	expectedEmojis := map[string]string{
		"eyes":          "eyes",
		"thinking_face": "thinking_face",
		"+1":            "thumbsup",
		"x":             "x",
		"-1":            "thumbsdown",
		"tada":          "tada",
	}

	for key, expectedValue := range expectedEmojis {
		actualValue, exists := emojiMap[key]
		if !exists {
			t.Errorf("expected emoji mapping for %s", key)
		}
		if actualValue != expectedValue {
			t.Errorf("expected %s -> %s, got %s", key, expectedValue, actualValue)
		}
	}
}

func TestAddCommentReaction_RequestBody(t *testing.T) {
	requestReceived := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true

		// Verify no body is sent for PUT request
		body, _ := io.ReadAll(r.Body)
		if len(body) > 0 {
			t.Errorf("expected empty body for reaction PUT request, got %d bytes", len(body))
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
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

	_ = client.AddCommentReaction("PROJ", "repo", 1, 123, "eyes")

	if !requestReceived {
		t.Fatalf("expected request to be sent")
	}
}
