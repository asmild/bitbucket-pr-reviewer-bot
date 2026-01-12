package config

import (
	"fmt"
	"os"
	"strings"
)

const (
	// EventTypePROpened triggers review automatically when PR is opened
	EventTypePROpened = "pr_opened"

	// EventTypeCommentAdded triggers review when comment is added with keyword
	EventTypeCommentAdded = "comment_added"
)

// SupportedEventTypes contains all supported webhook event types
var SupportedEventTypes = []string{
	EventTypePROpened,
	EventTypeCommentAdded,
}

// TriggeringEvent is an interface for all event types
type TriggeringEvent interface {
	GetType() string
	Validate() error
}

// RawTriggeringEvent is used for YAML unmarshaling
type RawTriggeringEvent struct {
	Type    string `yaml:"type"`
	Keyword string `yaml:"keyword,omitempty"`
}

// ToEvent converts RawTriggeringEvent to the TriggeringEvent implementation
func (r *RawTriggeringEvent) ToEvent() (TriggeringEvent, error) {
	switch r.Type {
	case EventTypePROpened:
		return &PROpenedEvent{}, nil
	case EventTypeCommentAdded:
		return &CommentAddedEvent{Keyword: r.Keyword}, nil
	default:
		return nil, fmt.Errorf("unknown event type: %s", r.Type)
	}
}

// PROpenedEvent represents a PR opened triggering event
type PROpenedEvent struct{}

func (e *PROpenedEvent) GetType() string { return EventTypePROpened }
func (e *PROpenedEvent) Validate() error { return nil }

// CommentAddedEvent represents a comment added triggering event with keyword
type CommentAddedEvent struct {
	Keyword string
}

func (e *CommentAddedEvent) GetType() string { return EventTypeCommentAdded }
func (e *CommentAddedEvent) Validate() error {
	if e.Keyword == "" {
		return fmt.Errorf("keyword is required for comment_added event")
	}
	return nil
}

// applyEventOverridesFromEnv parses env vars and applies overrides to RawEvents
func applyEventOverridesFromEnv(rawEvents []RawTriggeringEvent) []RawTriggeringEvent {
	const prefix = "BITBUCKET_PR_EVENT_"

	// Create map for events setup from config yaml
	eventMap := make(map[string]RawTriggeringEvent)
	for _, event := range rawEvents {
		eventMap[event.Type] = event
	}

	// Apply env var overrides
	for _, eventType := range SupportedEventTypes {
		envKey := prefix + strings.ToUpper(eventType)
		value, exists := os.LookupEnv(envKey)
		if !exists {
			continue
		}

		switch eventType {
		case EventTypePROpened:
			if value == "true" || value == "1" || strings.ToLower(value) == "yes" {
				eventMap[EventTypePROpened] = RawTriggeringEvent{Type: EventTypePROpened}
			} else {
				delete(eventMap, EventTypePROpened)
			}
		case EventTypeCommentAdded:
			if value != "" {
				eventMap[EventTypeCommentAdded] = RawTriggeringEvent{Type: EventTypeCommentAdded, Keyword: value}
			} else {
				delete(eventMap, EventTypeCommentAdded)
			}
		}
	}

	// Convert map to slice
	result := make([]RawTriggeringEvent, 0, len(eventMap))
	for _, event := range eventMap {
		result = append(result, event)
	}

	return result
}

// processTriggeringEvents converts RawEvents to Events
// If RawEvents is empty, keeps the default Events from getDefaultConfig()
func (c *Config) processTriggeringEvents() error {
	// If no RawEvents from YAML/env vars, keep the default Events
	if len(c.Bitbucket.RawEvents) == 0 {
		return nil
	}

	// Convert RawEvents to Events (replaces defaults)
	c.Bitbucket.Events = make([]TriggeringEvent, 0, len(c.Bitbucket.RawEvents))

	for _, raw := range c.Bitbucket.RawEvents {
		event, err := raw.ToEvent()
		if err != nil {
			return err
		}
		c.Bitbucket.Events = append(c.Bitbucket.Events, event)
	}

	return nil
}

// validateTriggeringEvents validates all configured triggering events
func (c *Config) validateTriggeringEvents() error {
	// Check for duplicate event types
	eventTypes := make(map[string]bool)
	for _, event := range c.Bitbucket.Events {
		eventType := event.GetType()
		if eventTypes[eventType] {
			return fmt.Errorf("duplicate event type: %s", eventType)
		}
		eventTypes[eventType] = true

		// Validate individual event
		if err := event.Validate(); err != nil {
			return fmt.Errorf("invalid %s event: %w", eventType, err)
		}
	}
	return nil
}
