# Architecture Explained

## ReviewService vs Application

### ReviewService (Domain Service - Business Logic)

**Location:** `internal/domain/services/review_service.go`

**Purpose:** Contains the **core business logic** for reviewing pull requests

**Responsibilities:**
1. Orchestrates the review workflow (clone repo → get profile → call AI → post comment)
2. Coordinates between different ports (VCS, Git, AI, Profiles)
3. Handles review errors and retries
4. Records metrics
5. Posts comments back to Bitbucket

**Level:** Domain layer (business logic, no infrastructure concerns)

**Used by:** Queue workers to actually perform reviews

### Application (Composition Root)

**Location:** `internal/app/application.go`

**Purpose:** **Wires everything together** and manages application lifecycle

**Responsibilities:**
1. Dependency injection (creates and connects all components)
2. HTTP server setup (webhook endpoints, health checks, metrics)
3. Application lifecycle (Start/Stop)
4. Configuration loading
5. Exposes getters for testing/external access

**Level:** Application layer (infrastructure, composition, lifecycle)

**Used by:** `main.go` to bootstrap the entire application

### The Relationship

```
main.go
  └─> Application (app)
       ├─> HTTP Server (handles webhooks)
       ├─> Queue (receives PRs from webhooks)
       └─> ReviewService (does actual review work)
            ├─> VCSClient (Bitbucket API)
            ├─> GitRepository (clone/fetch)
            ├─> AIReviewer (Claude CLI)
            └─> ProfileProvider (templates)
```

### In Hexagonal Architecture Terms

- **ReviewService** = Domain service (hexagon core - business logic)
- **Application** = Composition root (outside hexagon - wires adapters to ports)

The `Application` is the container that holds everything, while `ReviewService` is the brain that actually knows how to review code. The `Application` just connects the pieces and manages the HTTP server/queue lifecycle.

## Flow Example

1. **Bitbucket sends webhook** → HTTP handler in `Application`
2. **Webhook handler** → Enqueues PR to `Queue`
3. **Queue worker** → Calls `ReviewService.ReviewPullRequest()`
4. **ReviewService** → Orchestrates:
   - Clone repository via `GitRepository`
   - Get review template via `ProfileProvider`
   - Call AI via `AIReviewer`
   - Post comment via `VCSClient`
   - Record metrics via `MetricsCollector`
5. **Result** → Queue marks job complete, metrics updated

## Queue System

**Location:** `internal/adapters/queue/queue.go`

The Queue is a critical component that enables **asynchronous processing** of pull request reviews.

### How Queue.Start(ctx) Works

When `a.queue.Start(ctx)` is called during application startup (`application.go:198`), it:

1. **Marks the queue as running** (thread-safe with mutex)
2. **Spawns a background worker goroutine** (`queue.go:177`) that runs continuously
3. **Worker goroutine listens on three channels:**
   - `itemCh` - Receives PRs to review (from webhook enqueues)
   - `stopCh` - Receives stop signal for graceful shutdown
   - `ctx.Done()` - Context cancellation signal

### Worker Implementation

**Location:** `internal/adapters/queue/queue.go`

**`worker()` goroutine (line 262-279):**
- Infinite loop listening on channels
- Receives items from `itemCh` channel
- Calls `processItem()` for each PR

**`processItem()` method (line 282-332):**
- Line 301: Logs "Processing PR from queue" ← **work starts here**
- Line 309: Checks circuit breaker status
- Line 318: Calls `reviewService.ReviewPullRequest()` ← **actual review**
- Line 326: Logs "PR processed successfully" ← **work complete**

### Logging Timeline

1. **Webhook received** → `webhook.go` → `queue.Enqueue()` called
2. **Item enqueued** → `queue.go:129` → "PR added to queue"
3. **Worker picks up** → `queue.go:269` → item received from channel
4. **Processing starts** → `queue.go:301` → "Processing PR from queue"
5. **Review happens** → `queue.go:318` → ReviewService does the work
6. **Processing done** → `queue.go:326` → "PR processed successfully"

### Queue Processing Flow

```
Webhook → Enqueue(pr) → itemCh channel → Worker goroutine
                                        ↓
                                    processItem()
                                        ↓
                            Check circuit breaker
                                        ↓
                        ReviewService.ReviewPullRequest()
                                        ↓
                    ┌───────────────────┴────────────────┐
                    ↓                                    ↓
                Success                              Failure
                    ↓                                    ↓
            Remove from queue                    Retry with backoff?
                    ↓                                    ↓
            Metrics updated                      Yes: Requeue (5s, 10s, 15s)
                                                 No: Move to Dead Letter Queue
```

### Key Features

**Asynchronous Processing**
- Webhooks return immediately (200 OK)
- Reviews happen in background
- Multiple PRs can be queued simultaneously

**Circuit Breaker Protection**
- If too many reviews fail, circuit opens
- Prevents cascading failures
- Automatically closes after reset timeout

**Automatic Retries**
- Failed reviews are retried up to 3 times (configurable)
- Exponential backoff: 5s → 10s → 15s
- Only retries on retryable errors (network issues, timeouts)

**Dead Letter Queue (DLQ)**
- Non-retryable errors or exhausted retries → moved to DLQ
- DLQ items preserved for manual inspection
- Accessible via `/health` endpoint

**Graceful Shutdown**
- `Stop()` closes the stopCh channel
- Waits for current review to finish
- Uses sync.WaitGroup for coordination

### Configuration

```go
queue.Config{
    MaxRetries:  3,    // Maximum retry attempts
    ChannelSize: 100,  // Buffered channel size
}
```

### Why This Design?

1. **Non-blocking webhooks** - Bitbucket requires fast webhook responses
2. **Resilience** - Automatic retries handle transient failures
3. **Observability** - Failed items are tracked in DLQ
4. **Resource control** - Circuit breaker prevents resource exhaustion
5. **Clean shutdown** - No reviews are lost during application restart

## Profile System (Review Templates)

**Location:** `internal/adapters/profiles/provider.go`

Profiles are the **prompt templates** that tell Claude how to review the code. They're markdown files containing instructions for the AI.

### How Profiles Are Loaded and Passed to Claude

**Flow:** `ReviewService.performReview()` → `ProfileProvider.GetProfile()` → `Claude.Review()`

#### Step 1: Profile Selection (`review_service.go:110`)

```go
profile, err := s.profileProvider.GetProfile(ctx, pr)
```

#### Step 2: ProfileProvider.GetProfile() (`profiles/provider.go:56-94`)

The profile provider does the following:

**a) Resolves profile name** (line 58):
```
Resolution hierarchy:
1. Repository-specific override (e.g., PROJ1/my-repo → "critical-review")
2. Project-level profile (e.g., PROJ1 → "custom")
3. Default profile (e.g., "default")
```

**b) Loads profile file** (line 58, 67):
```go
profilePath := filepath.Join(p.directory, profileName+".md")
content, err := os.ReadFile(profilePath)
```
Example: `./profiles/default.md`

**c) Validates structure** (line 82):
- Checks for required sections: `Role:`, `Goal:`, `PR:`
- Checks minimum length (100 characters)

**d) Substitutes variables** (line 87):

Replaces placeholders with actual PR data:
- `{{prUrl}}` → PR URL (e.g., https://bitbucket.example.com/projects/PROJ/repos/repo/pull-requests/123)
- `{{title}}` → PR title
- `{{description}}` → PR description
- `{{author}}` → Author name
- `{{repository}}` → Repository slug
- `{{sourceBranch}}` → Source branch name
- `{{destinationBranch}}` → Destination branch name
- `{{repoCloneUrl}}` → Clone URL
- `{{projectKey}}` → Project key
- `{{prId}}` → PR ID number

#### Step 3: ReviewRequest Created (`review_service.go:119`)

```go
reviewReq := ports.NewReviewRequest(pr, repoPath, profile, s.reviewTimeout)
```

The processed profile is stored in `request.Template` field.

#### Step 4: Claude Execution (`claude/reviewer.go:149`)

```go
cmd.Stdin = bytes.NewBufferString(request.Template)
```

The profile template is **piped to Claude CLI via stdin**:

```bash
cd /path/to/cloned/repo
claude --model sonnet --output-format text < profile_content
```

Claude reads the prompt from stdin and reviews the code in the current directory.

### Complete Profile Flow Diagram

```
ReviewService.performReview()
    ↓
ProfileProvider.GetProfile()
    ↓
1. Resolve profile name
   (repo-specific → project → default)
    ↓
2. Read: ./profiles/{profileName}.md
    ↓
3. Validate structure
   (check Role:, Goal:, PR: sections)
    ↓
4. Substitute variables
   ({{prUrl}}, {{title}}, {{author}}, etc.)
    ↓
5. Return processed template string
    ↓
Create ReviewRequest(pr, repoPath, template, timeout)
    ↓
Claude.Review() → executeClaude()
    ↓
cmd.Dir = repoPath (set working directory)
cmd.Stdin = template content
    ↓
Execute: claude --model sonnet < template
    ↓
Claude reads prompt and reviews code in repo directory
    ↓
Return review comment + metrics
```

### Profile Configuration Example

**config.yaml:**
```yaml
profiles:
  directory: ./profiles
  default: default
  projects:
    PROJ1:
      profile: custom              # All repos in PROJ1 use "custom"
      repositories:
        critical-repo: security    # Override for specific repo
```

**File structure:**
```
profiles/
├── default.md             # Default review instructions
├── custom.md              # Custom instructions for PROJ1
└── security.md            # Security-focused for critical-repo
```

### Why This Design?

1. **Flexible** - Different review styles for different projects/repos
2. **Customizable** - Easy to modify prompts without code changes
3. **Context-aware** - Variables inject PR-specific information
4. **Validated** - Ensures prompts have required structure
5. **Simple** - Just markdown files, no complex templating engine

## Key Design Principles

### Separation of Concerns
- **Domain logic** (ReviewService) is isolated from infrastructure (Application)
- ReviewService has no idea about HTTP, queues, or how components are created
- Application has no idea about review logic, just wires things together

### Dependency Injection
- Application creates all concrete implementations
- ReviewService receives only interfaces (ports)
- Makes testing easy - mock the ports, test the logic

### Single Responsibility
- Application: Lifecycle management and composition
- ReviewService: Review orchestration logic
- Each adapter: One specific infrastructure concern

This design makes the codebase maintainable, testable, and follows hexagonal architecture principles.
