# Configuration Guide

This guide explains how to configure the PR Reviewer bot for your Bitbucket Data Center instance.

## Configuration Sources

The application supports configuration through multiple sources with the following priority (highest to lowest):

1. **Environment Variables** - Override YAML and defaults
2. **YAML Configuration File** - Loaded from `config.yaml` or specified path
3. **Default Values** - Built-in hardcoded defaults

## Configuration File

The main configuration file is YAML-based. By default, the application looks for `config.yaml` in the working directory.

### Minimal Configuration

At minimum, you need to provide Bitbucket credentials:

```yaml
bitbucket:
  user: your-username
  token: your-app-password
```

### Full Configuration Example

See `config.example.yaml` in the repository root for a complete example with all available options.

## Configuration Sections

### Server Configuration

Controls the HTTP server behavior.

```yaml
server:
  port: 8080  # (default: 8080)
```

**Environment Variable:** `PORT`

### Claude Configuration

Controls the Claude AI integration for PR reviews.

```yaml
claude:
  model: sonnet  # (default: sonnet)
  # Available models: "opus", "sonnet", "haiku"
  timeout_minutes: 10  # (default: 10)
  # Maximum time Claude can spend processing a single PR review
```

**Environment Variables:**
- `CLAUDE_MODEL` - Model to use for reviews
- `CLAUDE_TIMEOUT_CONFIG` - Timeout in minutes

### Bitbucket Configuration

Connects the bot to your Bitbucket Data Center instance.

```yaml
bitbucket:
  self-hosted: true  # (default: true)
  # Set to true if using Bitbucket Data Center / Server

  base_url: "http://bitbucket.example.com"  # (required if self-hosted)
  # Base URL of your Bitbucket instance

  user: "bot-username"  # (required)
  # Bitbucket username or email of the bot account

  token: "your-app-password"  # (required)
  # App password or personal access token with PR read/write permissions

  webhook_secret: "your-webhook-secret"  # (optional)
  # HMAC secret for webhook signature validation
  # Improves security by verifying webhook authenticity

  allowed_project_keys:  # (optional, default: all projects)
    - PROJ1
    - PROJ2
  # Restrict bot to specific projects
  # Leave empty or remove to accept all projects

  triggering_events:  # (default: comment_added with keyword "/review")
    # Defines which events trigger the bot to review a PR
    # Multiple events can be configured

    - type: pr_opened
      # Automatically review when PR is opened

    - type: comment_added
      keyword: "/review"
      # Review when comment contains keyword and mentions the bot
      # Example: User comments "@bot-username /review"
```

**Environment Variables:**
- `BITBUCKET_SELF_HOSTED` - Whether using self-hosted Bitbucket (true/false)
- `BITBUCKET_BASE_URL` - Base URL of Bitbucket instance (required if self-hosted)
- `BITBUCKET_USER` - Bitbucket username
- `BITBUCKET_TOKEN` - App password/token
- `BITBUCKET_WEBHOOK_SECRET` - Webhook secret
- `BITBUCKET_ALLOWED_PROJECT_KEYS` - Comma-separated project keys (e.g., "PROJ1,PROJ2")
- `BITBUCKET_EVENT_TYPE` - Single event type (for backward compatibility)
- `TRIGGER_KEYWORD` - Trigger keyword for reviews (used with comment_added)

### Profiles Configuration

Manages PR review profiles and customization per project/repository.

```yaml
profiles:
  directory: ./profiles  # (default: ./profiles)
  # Base directory where profile .md files are located

  default: default  # (default: default)
  # Default profile name without .md extension
  # Corresponds to ./profiles/default.md

  projects:  # (optional)
    # Per-project profile configurations
    CI:
      profile: custom  # Applied to all repos in CI project
      # Corresponds to ./profiles/custom.md
      repositories:
        critical-repo: critical-review  # Override for specific repo
        # Corresponds to ./profiles/critical-review.md
        important-repo: important-review

    INFRA:
      profile: infrastructure-review
      repositories:
        network-config: security-review

    DEV:
      # Can have just repo-level overrides without project-level profile
      repositories:
        experimental: lenient-review
```

**Profile Resolution Priority:**

1. **Repository-specific override** - `projects.<PROJECT>.repositories.<REPO>`
2. **Project-level profile** - `projects.<PROJECT>.profile`
3. **Global default** - `default`

If no project configuration exists for a repository, the global `default` profile is used.

**Environment Variables:**
- `PROFILES_DIRECTORY` - Profiles directory path
- `PROFILES_DEFAULT` - Default profile name (without .md extension)

**Note:** Profile projects/repositories are configured only through YAML, not environment variables.

**Profile File Structure:**
- Each profile is a markdown file (`.md`) in the profiles directory
- Example: `default` profile → `./profiles/default.md`
- The profile file contains AI review instructions

### Queue Configuration

Controls PR review queue behavior to manage concurrent processing and retries.

```yaml
queue:
  max_size: 100  # (default: 100)
  # Maximum number of PRs in queue
  # When full, new PRs will be rejected with 503 error
  # Manual triggers (@bot review) will get emoji reaction

  max_retries: 3  # (default: 3)
  # Maximum retry attempts for failed reviews
```

**Environment Variables:**
- `QUEUE_MAX_SIZE` - Maximum queue size
- `QUEUE_MAX_RETRIES` - Maximum retry attempts

### Circuit Breaker Configuration

Implements fault tolerance pattern to handle transient failures gracefully.

```yaml
circuit_breaker:
  failure_threshold: 3  # (default: 3)
  # Number of consecutive failures before opening circuit

  reset_timeout_ms: 30000  # (default: 30000)
  # Time in milliseconds to wait before attempting to reset circuit
```

**How it works:**

- **Closed** (normal): Requests pass through normally
- **Open** (failures): Requests fail fast without processing after threshold reached
- **Half-Open** (recovery): After timeout, one request attempts to reset the circuit

**Environment Variables:**
- `CB_FAILURE_THRESHOLD` - Failure threshold
- `CB_RESET_TIMEOUT_MS` - Reset timeout in milliseconds

### Metrics Configuration

Enables persistent metrics storage for monitoring bot activity.

```yaml
metrics:
  persistence:
    enabled: false  # (default: false)
    # Enable metrics persistence to track bot activity over time

    type: filesystem  # (default: filesystem)
    # Storage type: "filesystem" or "sqlite"

    path: ./metrics-storage  # (default: ./metrics-storage)
    # Path to store metrics data

    save_interval_ms: 30000  # (default: 30000)
    # Interval in milliseconds to persist metrics to storage
```

**Tracked Metrics:**

- PR reviews created
- PR reviews updated
- Successful reviews
- Failed reviews (with error type)
- LGTMs (Looks Good To Me)
- Issues found
- Review duration

**Environment Variables:**
- `METRICS_PERSISTENCE_ENABLED` - Enable/disable persistence (true/false)
- `METRICS_PERSISTENCE_TYPE` - Storage type
- `METRICS_PERSISTENCE_PATH` - Storage path
- `METRICS_PERSISTENCE_SAVE_INTERVAL_MS` - Save interval in milliseconds

### Rate Limiting Configuration

Controls webhook request rate limiting to protect the bot from being overwhelmed.

```yaml
rate_limit:
  enabled: false  # (default: false)
  # Enable rate limiting for webhook processing

  requests_per_minute: 60  # (default: 60)
  # Maximum number of webhook requests per minute
  # When limit is exceeded, webhooks are rejected with 429 Too Many Requests
```

**Environment Variables:**
- `RATE_LIMIT_ENABLED` - Enable/disable rate limiting (true/false)
- `RATE_LIMIT_REQUESTS_PER_MINUTE` - Maximum requests per minute

### Logging Configuration

Controls application logging behavior.

```yaml
logging:
  level: info  # (default: info)
  # Log level: "debug", "info", "warn", "error"

  file_retention_days: 30  # (default: 30)
  # Number of days to keep log files

  max_file_size: 20m  # (default: 20m)
  # Maximum size of a single log file before rotation
  # Format: "10m", "100m", "1g", etc.

  enable_console: true  # (default: true)
  # Log to console (stdout)

  enable_file: true  # (default: true)
  # Log to files in ./logs directory
```

**Environment Variables:**
- `LOG_LEVEL` - Log level
- `LOG_FILE_RETENTION_DAYS` - File retention days
- `LOG_MAX_FILE_SIZE` - Max file size
- `LOG_ENABLE_CONSOLE` - Enable console logging (true/false)
- `LOG_ENABLE_FILE` - Enable file logging (true/false)

## Environment Variables Reference

### Priority and Examples

When using environment variables, they override YAML configuration. For example:

```bash
# Override server port
export PORT=9000

# Override Claude model
export CLAUDE_MODEL=opus

# Override Bitbucket settings
export BITBUCKET_SELF_HOSTED=true
export BITBUCKET_BASE_URL=http://bitbucket.example.com
export BITBUCKET_USER=review-bot
export BITBUCKET_TOKEN=mytoken
export BITBUCKET_WEBHOOK_SECRET=mysecret

# Override project keys (comma-separated)
export BITBUCKET_ALLOWED_PROJECT_KEYS=PROJ1,PROJ2

# Override event type (for backward compatibility)
export BITBUCKET_EVENT_TYPE=pr_opened
export TRIGGER_KEYWORD="/review"

# Override profiles
export PROFILES_DIRECTORY=/custom/templates/path
export PROFILES_DEFAULT=my-profile

# Override logging
export LOG_LEVEL=debug
```

## Configuration Validation

The application validates required configuration on startup:

- `bitbucket.user` - Must be set (YAML or env var)
- `bitbucket.token` - Must be set (YAML or env var)
- `bitbucket.base_url` - Must be set if `bitbucket.self-hosted` is true
- `triggering_events` - Must contain valid event types ("pr_opened" or "comment_added")

If validation fails, the application logs an error and exits with status code 1.

## Loading Configuration

You can specify a custom configuration file path when starting the application:

```bash
# Using environment variable
export CONFIG_PATH=/etc/pr-reviewer/config.yaml
go run ./cmd/server/main.go

# Or modify the application code to use your path
```

By default, the application attempts to load `config.yaml` from the current working directory.

## Configuration Best Practices

1. **Start with Example** - Copy `config.example.yaml` as your base
2. **Use Environment Variables for Secrets** - Never commit credentials in YAML
3. **Document Your Setup** - Keep notes on what each project/repo template does
4. **Test Configuration** - Use the health endpoint to verify the service is running
5. **Monitor Logs** - Check logs for configuration issues on startup
6. **Set Appropriate Timeouts** - Balance between speed and comprehensive reviews
7. **Use Project Filtering** - Restrict to specific projects if not needed everywhere

## Common Configuration Scenarios

### Scenario 1: Single Project with Different Profiles

```yaml
profiles:
  default: default
  projects:
    CI:
      profile: custom
      repositories:
        critical-repo: critical-review
        important-repo: important-review
```

### Scenario 2: Multiple Projects with Defaults

```yaml
profiles:
  default: default
  projects:
    CI:
      profile: custom
    INFRA:
      profile: infrastructure-review
    DEV:
      profile: lenient-review
```

### Scenario 3: Automatic Review on PR Open

```yaml
bitbucket:
  triggering_events:
    - type: pr_opened
  # Automatically reviews all opened PRs
```

### Scenario 4: Manual Review via Comments

```yaml
bitbucket:
  triggering_events:
    - type: comment_added
      keyword: "/review"
  # Users comment with "/review" to trigger review
```

### Scenario 5: Both Automatic and Manual Triggers

```yaml
bitbucket:
  triggering_events:
    - type: pr_opened
    - type: comment_added
      keyword: "/review"
  # Reviews on PR open AND when users comment "/review"
```

## Troubleshooting

### Application Won't Start

Check the startup logs:
- Validate `bitbucket.user` and `bitbucket.token` are set
- Verify `bitbucket.base_url` is set if using self-hosted Bitbucket
- Verify `triggering_events` contains valid event types
- Check directory paths exist

### Profiles Not Found

- Verify `profiles.directory` path is correct
- Ensure profile `.md` files exist (e.g., `default.md`)
- Check file permissions

### Environment Variables Not Applied

- Verify variable names are exactly as documented
- Use uppercase for variable names
- Restart application after setting variables
