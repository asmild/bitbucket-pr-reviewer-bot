package config

import (
	"os"
	"testing"
)

func TestPROpenedEvent(t *testing.T) {
	event := &PROpenedEvent{}

	if event.GetType() != EventTypePROpened {
		t.Errorf("Expected type 'pr_opened', got '%s'", event.GetType())
	}

	if err := event.Validate(); err != nil {
		t.Errorf("Expected no validation error, got: %v", err)
	}
}

func TestCommentAddedEvent(t *testing.T) {
	tests := []struct {
		name        string
		keyword     string
		expectError bool
	}{
		{
			name:        "valid with keyword",
			keyword:     "/review",
			expectError: false,
		},
		{
			name:        "empty keyword",
			keyword:     "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &CommentAddedEvent{Keyword: tt.keyword}

			if event.GetType() != EventTypeCommentAdded {
				t.Errorf("Expected type 'comment_added', got '%s'", event.GetType())
			}

			err := event.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected validation error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

func TestRawTriggeringEventToEvent(t *testing.T) {
	tests := []struct {
		name        string
		raw         RawTriggeringEvent
		expectType  string
		expectError bool
	}{
		{
			name:        "pr_opened",
			raw:         RawTriggeringEvent{Type: EventTypePROpened},
			expectType:  EventTypePROpened,
			expectError: false,
		},
		{
			name:        "comment_added with keyword",
			raw:         RawTriggeringEvent{Type: EventTypeCommentAdded, Keyword: "/review"},
			expectType:  EventTypeCommentAdded,
			expectError: false,
		},
		{
			name:        "unknown type",
			raw:         RawTriggeringEvent{Type: "unknown_type"},
			expectType:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := tt.raw.ToEvent()

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error, got: %v", err)
				return
			}

			if event.GetType() != tt.expectType {
				t.Errorf("Expected type '%s', got '%s'", tt.expectType, event.GetType())
			}
		})
	}
}

func TestApplyEventOverridesFromEnv(t *testing.T) {
	tests := []struct {
		name      string
		rawEvents []RawTriggeringEvent
		envVars   map[string]string
		expected  map[string]RawTriggeringEvent
	}{
		{
			name: "no env vars - keep yaml",
			rawEvents: []RawTriggeringEvent{
				{Type: EventTypeCommentAdded, Keyword: "/analyze"},
			},
			envVars: map[string]string{},
			expected: map[string]RawTriggeringEvent{
				EventTypeCommentAdded: {Type: EventTypeCommentAdded, Keyword: "/analyze"},
			},
		},
		{
			name: "merge - yaml has comment_added, env adds pr_opened",
			rawEvents: []RawTriggeringEvent{
				{Type: EventTypeCommentAdded, Keyword: "/analyze"},
			},
			envVars: map[string]string{
				"BITBUCKET_PR_EVENT_PR_OPENED": "true",
			},
			expected: map[string]RawTriggeringEvent{
				EventTypeCommentAdded: {Type: EventTypeCommentAdded, Keyword: "/analyze"},
				EventTypePROpened:     {Type: EventTypePROpened},
			},
		},
		{
			name: "override - yaml has comment_added with /analyze, env overrides with /review",
			rawEvents: []RawTriggeringEvent{
				{Type: EventTypeCommentAdded, Keyword: "/analyze"},
			},
			envVars: map[string]string{
				"BITBUCKET_PR_EVENT_COMMENT_ADDED": "/review",
			},
			expected: map[string]RawTriggeringEvent{
				EventTypeCommentAdded: {Type: EventTypeCommentAdded, Keyword: "/review"},
			},
		},
		{
			name: "remove - yaml has pr_opened, env disables it",
			rawEvents: []RawTriggeringEvent{
				{Type: EventTypePROpened},
				{Type: EventTypeCommentAdded, Keyword: "/review"},
			},
			envVars: map[string]string{
				"BITBUCKET_PR_EVENT_PR_OPENED": "false",
			},
			expected: map[string]RawTriggeringEvent{
				EventTypeCommentAdded: {Type: EventTypeCommentAdded, Keyword: "/review"},
			},
		},
		{
			name:      "empty yaml - env adds pr_opened",
			rawEvents: []RawTriggeringEvent{},
			envVars: map[string]string{
				"BITBUCKET_PR_EVENT_PR_OPENED": "true",
			},
			expected: map[string]RawTriggeringEvent{
				EventTypePROpened: {Type: EventTypePROpened},
			},
		},
		{
			name: "complex - yaml has both, env disables pr_opened and changes keyword",
			rawEvents: []RawTriggeringEvent{
				{Type: EventTypePROpened},
				{Type: EventTypeCommentAdded, Keyword: "/analyze"},
			},
			envVars: map[string]string{
				"BITBUCKET_PR_EVENT_PR_OPENED":     "false",
				"BITBUCKET_PR_EVENT_COMMENT_ADDED": "/review",
			},
			expected: map[string]RawTriggeringEvent{
				EventTypeCommentAdded: {Type: EventTypeCommentAdded, Keyword: "/review"},
			},
		},
		{
			name:      "pr_opened with '1' and 'yes'",
			rawEvents: []RawTriggeringEvent{},
			envVars: map[string]string{
				"BITBUCKET_PR_EVENT_PR_OPENED": "1",
			},
			expected: map[string]RawTriggeringEvent{
				EventTypePROpened: {Type: EventTypePROpened},
			},
		},
		{
			name: "comment_added empty string - remove",
			rawEvents: []RawTriggeringEvent{
				{Type: EventTypeCommentAdded, Keyword: "/review"},
			},
			envVars: map[string]string{
				"BITBUCKET_PR_EVENT_COMMENT_ADDED": "",
			},
			expected: map[string]RawTriggeringEvent{},
		},
		{
			name:      "both events enabled via env",
			rawEvents: []RawTriggeringEvent{},
			envVars: map[string]string{
				"BITBUCKET_PR_EVENT_PR_OPENED":     "true",
				"BITBUCKET_PR_EVENT_COMMENT_ADDED": "/analyze",
			},
			expected: map[string]RawTriggeringEvent{
				EventTypePROpened:     {Type: EventTypePROpened},
				EventTypeCommentAdded: {Type: EventTypeCommentAdded, Keyword: "/analyze"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()

			// Set test env vars
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			result := applyEventOverridesFromEnv(tt.rawEvents)

			// Convert result to map for easier comparison
			resultMap := make(map[string]RawTriggeringEvent)
			for _, event := range result {
				resultMap[event.Type] = event
			}

			// Check length
			if len(resultMap) != len(tt.expected) {
				t.Errorf("expected %d events, got %d", len(tt.expected), len(resultMap))
			}

			// Check each expected event
			for eventType, expectedEvent := range tt.expected {
				resultEvent, exists := resultMap[eventType]
				if !exists {
					t.Errorf("expected event type %s not found in result", eventType)
					continue
				}
				if resultEvent.Type != expectedEvent.Type {
					t.Errorf("event type mismatch: expected %s, got %s", expectedEvent.Type, resultEvent.Type)
				}
				if resultEvent.Keyword != expectedEvent.Keyword {
					t.Errorf("keyword mismatch for %s: expected %s, got %s", eventType, expectedEvent.Keyword, resultEvent.Keyword)
				}
			}
		})
	}
}

func TestProcessTriggeringEvents(t *testing.T) {
	tests := []struct {
		name          string
		rawEvents     []RawTriggeringEvent
		defaultEvents []TriggeringEvent
		expectError   bool
		expectedCount int
	}{
		{
			name:      "empty rawEvents - keep defaults",
			rawEvents: []RawTriggeringEvent{},
			defaultEvents: []TriggeringEvent{
				&CommentAddedEvent{Keyword: "/review"},
			},
			expectError:   false,
			expectedCount: 1,
		},
		{
			name: "convert valid rawEvents",
			rawEvents: []RawTriggeringEvent{
				{Type: EventTypePROpened},
				{Type: EventTypeCommentAdded, Keyword: "/test"},
			},
			defaultEvents: []TriggeringEvent{
				&CommentAddedEvent{Keyword: "/review"},
			},
			expectError:   false,
			expectedCount: 2,
		},
		{
			name: "invalid event type",
			rawEvents: []RawTriggeringEvent{
				{Type: "invalid_type"},
			},
			defaultEvents: []TriggeringEvent{},
			expectError:   true,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Bitbucket: BitbucketConfig{
					RawEvents: tt.rawEvents,
					Events:    tt.defaultEvents,
				},
			}

			err := cfg.processTriggeringEvents()

			if tt.expectError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}

			if !tt.expectError {
				if len(cfg.Bitbucket.Events) != tt.expectedCount {
					t.Errorf("Expected %d events, got %d", tt.expectedCount, len(cfg.Bitbucket.Events))
				}
			}
		})
	}
}

func TestValidateTriggeringEvents(t *testing.T) {
	tests := []struct {
		name        string
		events      []TriggeringEvent
		expectError bool
	}{
		{
			name: "valid events",
			events: []TriggeringEvent{
				&PROpenedEvent{},
				&CommentAddedEvent{Keyword: "/review"},
			},
			expectError: false,
		},
		{
			name: "invalid - comment_added without keyword",
			events: []TriggeringEvent{
				&CommentAddedEvent{Keyword: ""},
			},
			expectError: true,
		},
		{
			name:        "empty events list",
			events:      []TriggeringEvent{},
			expectError: false,
		},
		{
			name: "duplicate event types - pr_opened",
			events: []TriggeringEvent{
				&PROpenedEvent{},
				&PROpenedEvent{},
			},
			expectError: true,
		},
		{
			name: "duplicate event types - comment_added",
			events: []TriggeringEvent{
				&CommentAddedEvent{Keyword: "/review"},
				&CommentAddedEvent{Keyword: "/analyze"},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Bitbucket: BitbucketConfig{
					Events: tt.events,
				},
			}

			err := cfg.validateTriggeringEvents()

			if tt.expectError && err == nil {
				t.Error("Expected validation error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}
