# Architecture Documentation

## Overview

This application has been refactored to follow **Hexagonal Architecture** (Ports & Adapters pattern) with clean separation of concerns, dependency injection, and production-grade best practices.

## Architecture Principles

1. **Hexagonal Architecture** - Business logic is isolated from infrastructure concerns
2. **Dependency Injection** - Manual constructor injection (no framework required)
3. **Interface-Driven Design** - All external dependencies are behind interfaces
4. **Context Propagation** - `context.Context` throughout for cancellation and tracing
5. **Structured Error Handling** - Custom error types with codes and retry hints
6. **Observability** - Comprehensive metrics and structured logging
7. **Resilience** - Circuit breaker, retry logic, and dead-letter queue

---

## Project Structure

```
├── cmd/server/
│   ├── main.go           # Old implementation (deprecated)
│   └── main_new.go       # New implementation with refactored architecture
│
├── internal/
│   ├── domain/           # Domain layer (business logic, no dependencies)
│   │   ├── errors/       # Custom error types with retry hints
│   │   ├── models/       # Business entities (PullRequest, ReviewResult, etc.)
│   │   ├── ports/        # Interfaces (11 ports defined)
│   │   └── services/     # Domain services (ReviewService)
│   │
│   ├── adapters/         # Infrastructure implementations (implements ports)
│   │   ├── bitbucket/    # VCS client + webhook parser
│   │   ├── claude/       # AI reviewer (Claude CLI)
│   │   ├── circuitbreaker/  # Circuit breaker implementation
│   │   ├── git/          # Git repository operations
│   │   ├── logger/       # Structured logging with log/slog
│   │   ├── metrics/      # Prometheus metrics collector
│   │   ├── profiles/     # Profile provider (selects which template to use)
│   │   └── queue/        # Review queue with DLQ
│   │
│   ├── infrastructure/   # Supporting infrastructure
│   │   ├── retry/        # Exponential backoff retry logic
│   │   └── ratelimit/    # Token bucket rate limiter
│   │
│   ├── app/              # Application layer (wires everything together)
│   │   ├── application.go  # Main application with DI
│   │   ├── config.go       # Config adapter
│   │   └── handlers/       # HTTP handlers
│   │       ├── webhook.go  # Webhook handler
│   │       └── health.go   # Health check handler
│   │
│   ├── config/           # Configuration management (old structure, still used)
│   └── [old packages]/   # Legacy packages (to be removed)
│
├── templates/            # Review prompt templates
├── examples/             # Example configurations
├── go.mod
└── Makefile
```

---

## Domain Layer

### Ports (Interfaces)

All external dependencies are abstracted behind interfaces in `internal/domain/ports/`:

1. **Logger** - Structured logging interface
2. **VCSClient** - Version control system operations (Bitbucket)
3. **VCSWebhookParser** - Webhook payload parsing
4. **GitRepository** - Git repository operations
5. **AIReviewer** - AI-powered code review
6. **ProfileProvider** - Review profile management (selects which template to use)
7. **MetricsCollector** - Metrics collection (Prometheus)
8. **CircuitBreaker** - Circuit breaker pattern (prevents cascade failures)
9. **ReviewQueue** - Async review queue
10. **HealthChecker** - Health checking (future)

### Models

Business entities in `internal/domain/models/`:

- **PullRequest** - Immutable PR data with validation
- **ReviewResult** - Review outcome with metrics
- **ReviewMetrics** - Structured metrics from AI review

### Services

Domain services in `internal/domain/services/`:

- **ReviewService** - Orchestrates the entire review process
  - Git operations
  - Profile loading (determines which template to use)
  - AI review
  - Comment posting
  - Error handling with emoji reactions

### Errors

Custom error system in `internal/domain/errors/`:

```go
type DomainError struct {
    Code      ErrorCode              // Error classification
    Message   string                 // Human-readable message
    Cause     error                  // Original error
    Retryable bool                   // Should this be retried?
    Metadata  map[string]interface{} // Additional context
}
```

**Error Codes:**
- `TIMEOUT`, `NETWORK_FAILURE`, `RATE_LIMIT_EXCEEDED`
- `VCS_UNAUTHORIZED`, `VCS_NOT_FOUND`, `VCS_API_ERROR`
- `GIT_CLONE_FAILED`, `GIT_UPDATE_FAILED`
- `REVIEWER_TIMEOUT`, `REVIEWER_FAILED`
- `TEMPLATE_NOT_FOUND`, `CIRCUIT_OPEN`, etc.

---

## Infrastructure Layer

### Adapters

Each adapter implements one or more ports from the domain layer.

#### Bitbucket Adapter

**Files:** `internal/adapters/bitbucket/`

- `client.go` - Implements `ports.VCSClient`
  - Context support for cancellation
  - Proper error wrapping with domain errors
  - Emoji tracking for status feedback

- `webhook.go` - Implements `ports.VCSWebhookParser`
  - Parses PR opened and comment events
  - Validates payload structure
  - Extracts PR data into domain models

#### Claude Adapter

**Files:** `internal/adapters/claude/`

- `reviewer.go` - Implements `ports.AIReviewer`
  - Executes Claude CLI with context timeout
  - Extracts metrics from JSON blocks
  - Handles both success and failure scenarios

#### Git Adapter

**Files:** `internal/adapters/git/`

- `repository.go` - Implements `ports.GitRepository`
  - Clone and update operations with context
  - **Credential sanitization** in logs (security!)
  - Graceful handling of update failures

#### Queue Adapter

**Files:** `internal/adapters/queue/`

- `queue.go` - Implements `ports.ReviewQueue`
  - Async processing with worker goroutine
  - **Dead Letter Queue (DLQ)** for failed items
  - Automatic retry with exponential backoff
  - Circuit breaker integration
  - Graceful shutdown

#### Others

- **Logger** - Thin adapter wrapping Go's `log/slog` to implement `ports.Logger`
- **Metrics** - Prometheus collector implementing `ports.MetricsCollector`
- **Circuit Breaker** - State machine with callbacks
- **Profiles** - Hierarchical profile resolution (project → repo → default)
  - A profile determines which template to use for a specific project/repository
  - Templates are the actual prompt.md files in `profiles/` directory

### Infrastructure Components

#### Retry Logic

**File:** `internal/infrastructure/retry/`

- Exponential backoff with jitter
- Configurable max attempts and delays
- Automatic detection of retryable errors
- Callback support for observability

```go
retrier := retry.New(retry.Config{
    MaxAttempts:  3,
    InitialDelay: 1 * time.Second,
    MaxDelay:     30 * time.Second,
    Multiplier:   2.0,
})

err := retrier.Execute(ctx, func() error {
    return someOperation()
})
```

#### Rate Limiter

**File:** `internal/infrastructure/ratelimit/`

- Token bucket algorithm
- Thread-safe implementation
- Context-aware waiting
- Helper functions: `PerSecond()`, `PerMinute()`, `PerHour()`

```go
limiter := ratelimit.PerSecond(10) // 10 requests per second
err := limiter.Execute(ctx, func() error {
    return apiCall()
})
```

---

## Application Layer

### Application Struct

**File:** `internal/app/application.go`

The `Application` struct is the heart of dependency injection:

```go
type Application struct {
    config           *Config
    reviewService    *services.ReviewService
    logger           ports.Logger
    vcsClient        ports.VCSClient
    webhookParser    ports.VCSWebhookParser
    gitRepo          ports.GitRepository
    aiReviewer       ports.AIReviewer
    profileProvider  ports.ProfileProvider
    metricsCollector ports.MetricsCollector
    circuitBreaker   ports.CircuitBreaker
    queue            ports.ReviewQueue
    server           *http.Server
}
```

**Lifecycle:**

1. `New(cfg)` - Initializes all dependencies (manual DI)
2. `Start(ctx)` - Starts queue and HTTP server
3. `Stop(ctx)` - Graceful shutdown with timeout

### HTTP Handlers

**Files:** `internal/app/handlers/`

#### Webhook Handler

- Validates webhook signatures (HMAC-SHA256)
- Parses and validates payloads
- Project authorization
- Queue enqueueing with context timeout
- Proper HTTP status codes

#### Health Handler

Returns JSON with:
- Overall status (healthy/unhealthy)
- Queue size and running state
- Circuit breaker state
- Component health breakdown

Example response:
```json
{
  "status": "healthy",
  "queueSize": 3,
  "queueRunning": true,
  "circuitBreaker": {
    "state": "closed",
    "isOpen": false
  },
  "components": {
    "queue": {"healthy": true, "message": "Queue is running"},
    "circuit_breaker": {"healthy": true, "message": "Circuit breaker is closed"}
  }
}
```

---

## Request Flow

### PR Opened Event

```
┌──────────────┐
│   Bitbucket  │
│   Webhook    │
└──────┬───────┘
       │
       ▼
┌──────────────────────────────────────────────────────┐
│ Webhook Handler                                      │
│  ├─ Validate signature                              │
│  ├─ Parse payload                                    │
│  ├─ Check project authorization                     │
│  └─ Extract base URL & set on VCS client            │
└──────┬───────────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────────┐
│ ReviewQueue.Enqueue()                                │
│  ├─ Create queue item                               │
│  ├─ Update metrics                                   │
│  └─ Send to channel                                  │
└──────┬───────────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────────┐
│ Queue Worker (goroutine)                             │
│  ├─ Check circuit breaker                           │
│  ├─ Call ReviewService.ReviewPullRequest()          │
│  ├─ Handle retries (max 3 attempts)                 │
│  └─ Move to DLQ if all retries fail                 │
└──────┬───────────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────────┐
│ ReviewService.ReviewPullRequest()                    │
│  ├─ Add "thinking_face" emoji                       │
│  ├─ GitRepository.GetOrUpdate()                     │
│  ├─ ProfileProvider.GetProfile()                    │
│  ├─ AIReviewer.Review()                             │
│  ├─ VCSClient.PostComment()                         │
│  ├─ Record metrics                                   │
│  └─ Add "thumbsup" emoji on success                 │
└──────────────────────────────────────────────────────┘
```

---

## Key Improvements

### 1. Testability (80%+ achievable)

**Before:**
- Tight coupling, global state
- Hard to mock dependencies
- ~40% test coverage

**After:**
- All dependencies are interfaces
- Easy to create mocks
- Constructor injection makes testing straightforward

Example test:
```go
func TestReviewService(t *testing.T) {
    mockVCS := &MockVCSClient{}
    mockGit := &MockGitRepository{}
    mockAI := &MockAIReviewer{}
    mockProfiles := &MockProfileProvider{}
    mockMetrics := &MockMetricsCollector{}
    mockLogger := &MockLogger{}

    service := services.NewReviewService(
        mockVCS, mockGit, mockAI, mockProfiles,
        mockMetrics, mockLogger, credentials, timeout,
    )

    // Test review process with mocks
}
```

### 2. Maintainability

- **Clear boundaries** between layers
- **Single Responsibility** - each adapter has one job
- **No circular dependencies**
- **Explicit dependencies** via constructors
- **Self-documenting** code with interfaces

### 3. Production Readiness

#### Resilience
- ✅ Circuit breaker prevents cascade failures
- ✅ Retry logic with exponential backoff
- ✅ Dead Letter Queue for failed items
- ✅ Graceful shutdown with context timeout
- ✅ Context propagation for cancellation

#### Security
- ✅ Credential sanitization in logs
- ✅ Webhook signature validation
- ✅ Rate limiting support
- ✅ Input validation in domain models
- ✅ No secrets in error messages

#### Observability
- ✅ Structured logging with context fields
- ✅ Prometheus metrics for all operations
- ✅ Circuit breaker state transitions tracked
- ✅ Queue size monitoring
- ✅ Error categorization and tracking

---

## Configuration

The application uses the existing `config.yaml` structure but converts it internally to `app.Config`.

**Config precedence:**
1. Environment variables (highest)
2. YAML file
3. Defaults (lowest)

**Search paths:**
1. `$CONFIG_PATH` environment variable
2. `./config.yaml`
3. `~/.bb-pr-reviewer/config.yaml`
4. `/etc/bb-pr-reviewer/config.yaml`

---

## Running the Application

### New Architecture

```bash
# Build
go build -o bin/pr-reviewer-new ./cmd/server/main_new.go

# Run with default config
./bin/pr-reviewer-new

# Run with custom config
CONFIG_PATH=/path/to/config.yaml ./bin/pr-reviewer-new
```

### Old Architecture (deprecated)

```bash
go build -o bin/pr-reviewer ./cmd/server/main.go
./bin/pr-reviewer
```

---

## Migration Path

1. ✅ **Phase 1: Foundation** - Domain layer, errors, interfaces
2. ✅ **Phase 2: Adapters** - All infrastructure implementations
3. ✅ **Phase 3: Application** - DI container, handlers
4. ✅ **Phase 4: Compilation** - Fixed all imports and syntax
5. ⏳ **Phase 5: Testing** - Unit tests with mocks
6. ⏳ **Phase 6: Deployment** - Test in staging, roll out
7. ⏳ **Phase 7: Cleanup** - Remove old code

---

## Next Steps

### Immediate
1. **Test the new binary** with your existing config
2. **Verify** it works with real webhooks
3. **Monitor** metrics and logs

### Short-term
1. **Write unit tests** for domain services
2. **Write integration tests** for adapters
3. **Add more metrics** as needed
4. **Performance testing** under load

### Long-term
1. **Remove old code** once stable
2. **Add distributed tracing** (OpenTelemetry)
3. **Kubernetes deployment** manifests
4. **Helm chart** for easy deployment

---

## Troubleshooting

### Logs
Structured logs include context:
```
level=info msg="Starting PR review" project=PROJ repo=my-repo pr_id=123 author=john
level=info msg="Repository cloned successfully" project=PROJ repo=my-repo path=./projects/PROJ/my-repo
level=info msg="AI review completed" model=claude-sonnet-4 duration=45.2s pr_id=123
```

### Metrics
Available at `/metrics`:
- `webhook_received_total{event_type}`
- `review_started_total{project}`
- `review_completed_total{project,status}`
- `review_failed_total{project,error_type}`
- `review_duration_seconds{project}`
- `queue_size`
- `circuit_breaker_state{name}`

### Health Check
```bash
curl http://localhost:8080/health | jq
```

---

## Architecture Decision Records (ADRs)

### Why Hexagonal Architecture?
- **Testability** - Easy to mock external dependencies
- **Maintainability** - Clear boundaries, easy to modify
- **Flexibility** - Can swap implementations (e.g., different VCS)

### Why Manual DI over Framework?
- **Simplicity** - No magic, explicit wiring
- **No dependencies** - One less thing to manage
- **Type safety** - Compiler catches errors
- **Debugging** - Easy to trace initialization

### Why Dead Letter Queue?
- **Reliability** - Don't lose failed reviews
- **Debugging** - Can inspect failed items
- **Reprocessing** - Can manually retry later

### Why Context Everywhere?
- **Cancellation** - Stop long-running operations
- **Timeouts** - Prevent resource leaks
- **Tracing** - Future distributed tracing support

---

## Contributors

Refactored by Claude Code (Anthropic) following Go best practices and clean architecture principles.

## License

[Same as original project]
