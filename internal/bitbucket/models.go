package bitbucket

import (
	"net/http"
	"sync"
)

// Client represents a Bitbucket API client
type Client struct {
	baseURL      string
	user         string
	token        string
	client       *http.Client
	emojiTracker *EmojiTracker
}

// EmojiTracker manages emoji reactions per comment
type EmojiTracker struct {
	current map[int]string
	mu      sync.Mutex
}

// WebhookPayload represents the Bitbucket Data Center webhook payload
type WebhookPayload struct {
	PullRequest PullRequest `json:"pullRequest"`
	Repository  Repository  `json:"repository"`
	Comment     *Comment    `json:"comment"`
}

type PullRequest struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Author      Author  `json:"author"`
	FromRef     Ref     `json:"fromRef"`
	ToRef       Ref     `json:"toRef"`
	Links       PRLinks `json:"links"`
}

type Author struct {
	User User `json:"user"`
}

type User struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

type Ref struct {
	ID           string     `json:"id"`
	DisplayID    string     `json:"displayId"`
	LatestCommit string     `json:"latestCommit"`
	Repository   Repository `json:"repository"`
}

type PRLinks struct {
	Self []Link `json:"self"`
}

type Repository struct {
	Slug    string    `json:"slug"`
	Name    string    `json:"name"`
	Project *Project  `json:"project"`
	Links   RepoLinks `json:"links"`
}

type Project struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type RepoLinks struct {
	Clone []CloneLink `json:"clone"`
	Self  []Link      `json:"self"`
}

type Link struct {
	Href string `json:"href"`
}

type CloneLink struct {
	Name string `json:"name"`
	Href string `json:"href"`
}

type Comment struct {
	ID      int    `json:"id"`
	Text    string `json:"text"`
	Author  Author `json:"author"`
	Version int    `json:"version"`
}
