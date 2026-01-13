package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"time"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/errors"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/models"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/ports"
)

// Common model aliases for Claude CLI
// See: claude --help for full list
const (
	ModelSonnet = "sonnet" // Latest Sonnet model (recommended for most use cases)
	ModelOpus   = "opus"   // Latest Opus model (most capable, slower)
	ModelHaiku  = "haiku"  // Latest Haiku model (fastest, most economical)
)

// Reviewer implements ports.AIReviewer using Claude CLI
type Reviewer struct {
	modelName string
	logger    ports.Logger
}

// Config holds Claude reviewer configuration
type Config struct {
	// ModelName can be:
	// - Model alias: "sonnet", "opus", "haiku" (recommended)
	// - Specific version: "claude-sonnet-4-5-20250929"
	// Defaults to "sonnet" if not specified
	ModelName string
}

// NewReviewer creates a new Claude reviewer
func NewReviewer(cfg Config, logger ports.Logger) *Reviewer {
	if cfg.ModelName == "" {
		cfg.ModelName = ModelSonnet
	}

	return &Reviewer{
		modelName: cfg.ModelName,
		logger:    logger,
	}
}

// Review performs a code review on a pull request
func (r *Reviewer) Review(ctx context.Context, request *ports.ReviewRequest) (*models.ReviewResult, error) {
	result := models.NewReviewResult("", r.modelName)

	r.logger.Info("Starting AI review",
		"model", r.modelName,
		"repo_path", request.RepositoryPath,
		"pr_id", request.PullRequest.ID,
	)

	// Create context with timeout
	reviewCtx, cancel := context.WithTimeout(ctx, time.Duration(request.Timeout)*time.Second)
	defer cancel()

	// Execute Claude CLI
	execResult, err := r.executeClaude(reviewCtx, request)
	if err != nil {
		result.Complete()

		if reviewCtx.Err() == context.DeadlineExceeded {
			return result, errors.Wrap(errors.ErrorCodeReviewerTimeout,
				fmt.Sprintf("review timed out after %d seconds", request.Timeout),
				err,
			).WithMetadata("timeout_seconds", request.Timeout)
		}

		return result, errors.Wrap(errors.ErrorCodeReviewerFailed,
			"claude execution failed",
			err,
		)
	}

	// Extract metrics from output
	reviewMetrics, err := r.extractMetrics(execResult.Result)
	if err != nil {
		r.logger.Warn("Failed to extract metrics from Claude output",
			"error", err,
		)
		// Continue without metrics - we still have the comment
	}

	// Set result
	result.Comment = execResult.Result
	if reviewMetrics != nil {
		result.WithMetrics(*reviewMetrics)
	}

	// Set cost/usage metrics from Claude response (store full precision)
	result.SetUsage(execResult.Usage.InputTokens, execResult.Usage.OutputTokens, execResult.CostUSD)

	result.Complete()

	r.logger.Info("AI review completed",
		"model", r.modelName,
		"duration", result.Duration,
		"pr_id", request.PullRequest.ID,
		"input_tokens", execResult.Usage.InputTokens,
		"output_tokens", execResult.Usage.OutputTokens,
		"cost_usd", execResult.CostUSD,
	)

	return result, nil
}

// GetModelName returns the name of the AI model being used
func (r *Reviewer) GetModelName() string {
	return r.modelName
}

// IsAvailable checks if the AI reviewer is available
func (r *Reviewer) IsAvailable(ctx context.Context) error {
	// Check if Claude CLI is installed
	cmd := exec.CommandContext(ctx, "claude", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(errors.ErrorCodeReviewerFailed,
			"claude CLI is not available",
			err,
		).WithMetadata("output", string(output))
	}

	r.logger.Debug("Claude CLI is available",
		"version_output", string(output),
	)

	return nil
}

// claudeJSONResponse represents the JSON output from Claude CLI
type claudeJSONResponse struct {
	Result     string      `json:"result"`
	IsError    bool        `json:"is_error"`
	CostUSD    float64     `json:"total_cost_usd"`
	DurationMS int64       `json:"duration_ms"`
	NumTurns   int         `json:"num_turns"`
	Usage      claudeUsage `json:"usage"`
}

// claudeUsage represents token usage from Claude CLI
type claudeUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

// executeClaude executes the Claude CLI with the given prompt
func (r *Reviewer) executeClaude(ctx context.Context, request *ports.ReviewRequest) (*claudeJSONResponse, error) {
	// Prepare command with JSON output format for cost tracking
	cmd := exec.CommandContext(ctx, "claude",
		"--dangerously-skip-permissions",
		"--model", r.modelName,
		"--output-format", "json",
	)
	cmd.Dir = request.RepositoryPath

	// Set up stdin with template prompt
	cmd.Stdin = bytes.NewBufferString(request.Template)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute
	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			r.logger.Warn("Claude CLI execution timed out",
				"duration", duration,
			)
			return nil, ctx.Err()
		}

		stdoutStr := stdout.String()
		stderrStr := stderr.String()

		// Log at DEBUG level with details - full error will be logged at higher level
		r.logger.Debug("Claude CLI command failed",
			"error", err,
			"duration", duration,
			"has_stdout", stdoutStr != "",
			"has_stderr", stderrStr != "",
		)

		// Include both outputs in error message for debugging
		return nil, fmt.Errorf("claude execution failed: %w, stdout: %s, stderr: %s", err, stdoutStr, stderrStr)
	}

	// Parse JSON response
	var response claudeJSONResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		r.logger.Warn("Failed to parse Claude JSON response, falling back to raw output",
			"error", err,
		)
		// Fallback to raw output if JSON parsing fails
		return &claudeJSONResponse{
			Result: stdout.String(),
		}, nil
	}

	if response.IsError {
		return nil, fmt.Errorf("claude returned error: %s", response.Result)
	}

	r.logger.Debug("Claude execution completed",
		"input_tokens", response.Usage.InputTokens,
		"output_tokens", response.Usage.OutputTokens,
		"cost_usd", response.CostUSD,
	)

	return &response, nil
}

// extractMetrics extracts metrics from Claude output
func (r *Reviewer) extractMetrics(output string) (*models.ReviewMetrics, error) {
	// Look for JSON code block containing metrics
	re := regexp.MustCompile("```json\\s*({[\\s\\S]*?})\\s*```")
	matches := re.FindStringSubmatch(output)

	if len(matches) < 2 {
		return nil, fmt.Errorf("no JSON metrics found in output")
	}

	// Parse the JSON
	var rawMetrics struct {
		IsLGTM             bool    `json:"isLgtm"`
		IssueCount         int     `json:"issueCount"`
		CriticalIssues     int     `json:"criticalIssues"`
		WarningIssues      int     `json:"warningIssues"`
		SuggestionCount    int     `json:"suggestionCount"`
		FilesChanged       int     `json:"filesChanged"`
		LinesAdded         int     `json:"linesAdded"`
		LinesRemoved       int     `json:"linesRemoved"`
		SecurityScore      float64 `json:"securityScore"`
		CodeQuality        float64 `json:"codeQuality"`
		TestCoverage       float64 `json:"testCoverage"`
		ComplexityScore    float64 `json:"complexityScore"`
		IsReviewFailed     bool    `json:"isReviewFailed"`
		FailedReviewReason string  `json:"failedReviewReason"`
	}

	if err := json.Unmarshal([]byte(matches[1]), &rawMetrics); err != nil {
		return nil, fmt.Errorf("failed to parse JSON metrics: %w", err)
	}

	// Check if review failed
	if rawMetrics.IsReviewFailed {
		return nil, fmt.Errorf("review failed: %s", rawMetrics.FailedReviewReason)
	}

	// Convert to domain metrics
	metrics := &models.ReviewMetrics{
		FilesChanged:    rawMetrics.FilesChanged,
		LinesAdded:      rawMetrics.LinesAdded,
		LinesRemoved:    rawMetrics.LinesRemoved,
		IssuesFound:     rawMetrics.IssueCount,
		CriticalIssues:  rawMetrics.CriticalIssues,
		WarningIssues:   rawMetrics.WarningIssues,
		SuggestionCount: rawMetrics.SuggestionCount,
	}

	// Add optional scores if present
	if rawMetrics.SecurityScore > 0 {
		metrics.SecurityScore = &rawMetrics.SecurityScore
	}
	if rawMetrics.CodeQuality > 0 {
		metrics.CodeQuality = &rawMetrics.CodeQuality
	}
	if rawMetrics.TestCoverage > 0 {
		metrics.TestCoverage = &rawMetrics.TestCoverage
	}
	if rawMetrics.ComplexityScore > 0 {
		metrics.ComplexityScore = &rawMetrics.ComplexityScore
	}

	return metrics, nil
}
