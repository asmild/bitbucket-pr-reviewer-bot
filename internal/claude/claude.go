package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"time"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/config"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/git"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/logger"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/metrics"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/templates"
	"github.com/asmild/bitbucket-pr-reviewer-bot/pkg/models"
)

const (
	ErrorTypeTimeout = "timeout"
	ErrorTypeGit     = "git_error"
	ErrorTypeClaude  = "claude_reported"
	ErrorTypeUnknown = "unknown"
)

// ReviewOutput represents the JSON metrics output from Claude
type ReviewOutput struct {
	IsLGTM             bool   `json:"isLgtm"`
	IssueCount         int    `json:"issueCount"`
	IsReviewFailed     bool   `json:"isReviewFailed"`
	FailedReviewReason string `json:"failedReviewReason"`
}

// ProcessPullRequest processes a pull request with Claude review
func ProcessPullRequest(cfg *config.Config, prData *models.PRData) error {
	startTime := time.Now()

	logger.WithFields(map[string]interface{}{
		"repository":    prData.Repository,
		"sourceBranch":  prData.SourceBranch,
		"prTitle":       prData.Title,
	}).Info("Processing pull request")

	// Ensure repository exists and is up to date
	projectPath := git.GetProjectPath(prData.ProjectKey, prData.Repository)
	if err := git.EnsureRepository(
		prData.RepoCloneURL,
		projectPath,
		prData.SourceBranch,
		cfg.Bitbucket.User,
		cfg.Bitbucket.Token,
	); err != nil {
		duration := time.Since(startTime).Seconds()
		metrics.RecordReviewFailure(prData.Repository, ErrorTypeGit, duration)
		return fmt.Errorf("failed to ensure repository: %w", err)
	}

	// Load and process template
	prompt, err := templates.LoadAndProcessTemplate(cfg, prData.ProjectKey, prData.Repository, prData)
	if err != nil {
		duration := time.Since(startTime).Seconds()
		metrics.RecordReviewFailure(prData.Repository, ErrorTypeUnknown, duration)
		return fmt.Errorf("failed to load template: %w", err)
	}

	// Execute Claude CLI
	output, err := executeClaude(cfg, projectPath, prompt)
	if err != nil {
		duration := time.Since(startTime).Seconds()
		if err == context.DeadlineExceeded {
			metrics.RecordReviewFailure(prData.Repository, ErrorTypeTimeout, duration)
			return fmt.Errorf("claude review timed out")
		}
		metrics.RecordReviewFailure(prData.Repository, ErrorTypeUnknown, duration)
		return fmt.Errorf("claude execution failed: %w", err)
	}

	// Extract metrics from output
	reviewOutput, err := extractMetrics(output)
	if err != nil {
		logger.Warnf("Failed to extract metrics from Claude output: %v", err)
		reviewOutput = &ReviewOutput{
			IsReviewFailed:     true,
			FailedReviewReason: "Failed to parse Claude output",
		}
	}

	duration := time.Since(startTime).Seconds()

	// Record metrics
	if reviewOutput.IsReviewFailed {
		metrics.RecordReviewFailure(prData.Repository, ErrorTypeClaude, duration)
		return fmt.Errorf("claude review failed: %s", reviewOutput.FailedReviewReason)
	}

	metrics.RecordReviewSuccess(prData.Repository, duration)

	if reviewOutput.IsLGTM {
		metrics.RecordLGTM(prData.Repository)
	}

	if reviewOutput.IssueCount > 0 {
		metrics.RecordIssuesFound(prData.Repository, reviewOutput.IssueCount)
	}

	logger.WithFields(map[string]interface{}{
		"repository":  prData.Repository,
		"isLgtm":      reviewOutput.IsLGTM,
		"issueCount":  reviewOutput.IssueCount,
		"duration":    duration,
	}).Info("Pull request processed successfully")

	return nil
}

// executeClaude executes the Claude CLI with the given prompt
func executeClaude(cfg *config.Config, workDir, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Claude.TimeoutMinutes)*time.Minute)
	defer cancel()

	logger.Debug("Executing Claude CLI")

	// Prepare command
	cmd := exec.CommandContext(ctx, "claude",
		"--dangerously-skip-permissions",
		"--model", cfg.Claude.Model,
		"--output-format", "text",
	)
	cmd.Dir = workDir

	// Set up stdin with prompt
	cmd.Stdin = bytes.NewBufferString(prompt)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute
	err := cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			logger.Warn("Claude CLI execution timed out")
			return "", context.DeadlineExceeded
		}
		logger.Errorf("Claude CLI execution failed: %v, stderr: %s", err, stderr.String())
		return "", err
	}

	output := stdout.String()
	logger.Debugf("Claude CLI output length: %d bytes", len(output))

	return output, nil
}

// extractMetrics extracts JSON metrics from Claude output
func extractMetrics(output string) (*ReviewOutput, error) {
	// Look for JSON code block
	re := regexp.MustCompile("```json\\s*({[\\s\\S]*?})\\s*```")
	matches := re.FindStringSubmatch(output)

	if len(matches) < 2 {
		return nil, fmt.Errorf("no JSON metrics found in output")
	}

	var result ReviewOutput
	if err := json.Unmarshal([]byte(matches[1]), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON metrics: %w", err)
	}

	return &result, nil
}

// CreateTempPromptFile creates a temporary file with the prompt
func CreateTempPromptFile(prompt string) (string, error) {
	tmpFile, err := os.CreateTemp("", "claude-prompt-*.md")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(prompt); err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	return tmpFile.Name(), nil
}
