package bitbucket

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoRequest_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify authentication
		user, pass, ok := r.BasicAuth()
		if !ok || user != "testuser" || pass != "testtoken" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Verify headers
		if r.Header.Get("Content-Type") != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := &Client{
		user:   "testuser",
		token:  "testtoken",
		client: &http.Client{},
	}

	resp, err := client.doRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(body, []byte("ok")) {
		t.Fatalf("expected response to contain 'ok'")
	}
}

func TestDoRequest_WithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if bytes.Contains(body, []byte("test")) {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := &Client{
		user:   "testuser",
		token:  "testtoken",
		client: &http.Client{},
	}

	payload := []byte(`{"test": true}`)
	resp, err := client.doRequest("POST", server.URL, payload)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestDoRequest_InvalidURL(t *testing.T) {
	client := &Client{
		user:   "testuser",
		token:  "testtoken",
		client: &http.Client{},
	}

	_, err := client.doRequest("GET", "://invalid", nil)
	if err == nil {
		t.Fatalf("expected error for invalid URL")
	}
}

func TestGet_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"key":"value"}`))
	}))
	defer server.Close()

	client := &Client{
		user:   "testuser",
		token:  "testtoken",
		client: &http.Client{},
	}

	body, err := client.get(server.URL)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var result map[string]string
	json.Unmarshal(body, &result)
	if result["key"] != "value" {
		t.Fatalf("expected key='value', got %v", result)
	}
}

func TestGet_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer server.Close()

	client := &Client{
		user:   "testuser",
		token:  "testtoken",
		client: &http.Client{},
	}

	_, err := client.get(server.URL)
	if err == nil {
		t.Fatalf("expected error for 404 response")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("404")) {
		t.Fatalf("expected error to contain '404'")
	}
}

func TestPost_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"created":true}`))
	}))
	defer server.Close()

	client := &Client{
		user:   "testuser",
		token:  "testtoken",
		client: &http.Client{},
	}

	payload := []byte(`{"text":"test"}`)
	body, err := client.post(server.URL, payload)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var result map[string]bool
	json.Unmarshal(body, &result)
	if !result["created"] {
		t.Fatalf("expected created=true")
	}
}

func TestPost_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer server.Close()

	client := &Client{
		user:   "testuser",
		token:  "testtoken",
		client: &http.Client{},
	}

	_, err := client.post(server.URL, []byte{})
	if err == nil {
		t.Fatalf("expected error for 500 response")
	}
}

func TestPut_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"updated":true}`))
	}))
	defer server.Close()

	client := &Client{
		user:   "testuser",
		token:  "testtoken",
		client: &http.Client{},
	}

	body, err := client.put(server.URL, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var result map[string]bool
	json.Unmarshal(body, &result)
	if !result["updated"] {
		t.Fatalf("expected updated=true")
	}
}

func TestDelete_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := &Client{
		user:   "testuser",
		token:  "testtoken",
		client: &http.Client{},
	}

	err := client.delete(server.URL)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDelete_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer server.Close()

	client := &Client{
		user:   "testuser",
		token:  "testtoken",
		client: &http.Client{},
	}

	err := client.delete(server.URL)
	if err == nil {
		t.Fatalf("expected error for 404 response")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("404")) {
		t.Fatalf("expected error to contain '404'")
	}
}

func TestHandleResponse_Success(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(`{"test":"data"}`)),
	}

	client := &Client{}
	body, err := client.handleResponse(resp)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !bytes.Contains(body, []byte("test")) {
		t.Fatalf("expected body to contain 'test'")
	}
}

func TestHandleResponse_BadStatusCode(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(bytes.NewBufferString("bad request")),
	}

	client := &Client{}
	_, err := client.handleResponse(resp)
	if err == nil {
		t.Fatalf("expected error for 400 status")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("400")) {
		t.Fatalf("expected error to contain '400'")
	}
}

func TestHandleResponse_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		shouldFail bool
	}{
		{"Status 200", http.StatusOK, false},
		{"Status 201", http.StatusCreated, false},
		{"Status 204", http.StatusNoContent, false},
		{"Status 299", 299, false},
		{"Status 300", http.StatusMultipleChoices, true},
		{"Status 400", http.StatusBadRequest, true},
		{"Status 500", http.StatusInternalServerError, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Body:       io.NopCloser(bytes.NewBufferString("test")),
			}

			client := &Client{}
			_, err := client.handleResponse(resp)

			if tt.shouldFail && err == nil {
				t.Fatalf("expected error for status %d", tt.statusCode)
			}
			if !tt.shouldFail && err != nil {
				t.Fatalf("expected no error for status %d, got %v", tt.statusCode, err)
			}
		})
	}
}
