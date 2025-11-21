package models

// PRData represents extracted pull request data
// This is the core domain model shared across all packages
type PRData struct {
	Title             string
	Description       string
	Author            string
	SourceBranch      string
	DestinationBranch string
	PRUrl             string
	Repository        string
	RepoCloneURL      string
	PRID              int
	CommentID         int
	ProjectKey        string
}
