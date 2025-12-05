# Bitbucket PR Reviewer Bot

An automated pull request reviewer for Bitbucket Data Center that uses Claude AI to provide code reviews.

## Features

- Automated PR reviews powered by Claude AI
- Dual trigger modes: automatic on PR open or manual via comments
- Customizable review profiles per project/repository
- Circuit breaker pattern for fault tolerance
- Prometheus metrics and health monitoring
- Webhook signature validation
- Queue management with retry logic

## Quick Start

### Prerequisites

- Go 1.25+
- Claude CLI installed and configured
- Bitbucket Data Center instance
- Bitbucket app password or personal access token

### Installation

1. Clone and build:
```bash
git clone https://github.com/asmild/bitbucket-pr-reviewer-bot.git
cd bitbucket-pr-reviewer-bot
make build-local
```

2. Configure:
```bash
cp config.example.yaml config.yaml
# Edit config.yaml with your settings
```

Minimal required configuration:
```yaml
bitbucket:
  self-hosted: true
  base_url: "https://bitbucket.example.com"
  user: your-bot-username
  token: your-app-password
```

3. Run:
```bash
./bin/bb-pr-reviewer
```

The application will:
- Validate all dependencies (Claude CLI, Bitbucket connection, git, profiles)
- Start HTTP server (default port 8080)
- Start queue workers
- Exit immediately if port is in use or dependencies are missing

### Bitbucket Webhook Setup

In Bitbucket Data Center:
1. Go to Repository/Project Settings → Webhooks
2. Add webhook:
   - URL: `https://your-server.com/webhook/bitbucket`
   - Secret: (optional, for HMAC validation)
   - Events: `Pull request opened` and/or `Pull request comment added`

## Usage

**Automatic mode** (`event_type: "pr_opened"`): Reviews PRs automatically when opened

**Manual mode** (`event_type: "comment_added"`): Trigger with comment:
```
@bot-username /review
```

## Configuration

See [Configuration Guide](docs/configuration.md) for all options. Configuration via YAML file or environment variables (env vars take precedence).

## Profiles

Profiles are markdown files that define review instructions for Claude. See [Profiles Guide](docs/profiles.md) for details on creating custom profiles per project or repository.

## API Endpoints

- `GET /health` - Health check (queue status, circuit breaker state)
- `GET /metrics` - Prometheus metrics
- `POST /webhook/bitbucket` - Webhook receiver for Bitbucket events

## Monitoring

Prometheus metrics available at `/metrics`. See [Metrics Guide](docs/metrics.md) for details.

## Architecture

```
├── cmd/server/                  # Application entry point
├── internal/
│   ├── adapters/               # External integrations (hexagonal architecture)
│   │   ├── bitbucket-dc/      # Bitbucket Data Center client & webhook parser
│   │   ├── claude/            # Claude AI reviewer
│   │   ├── git/               # Git repository operations
│   │   ├── logger/            # Structured logging
│   │   ├── metrics/           # Prometheus metrics collector
│   │   ├── profiles/          # Profile provider
│   │   ├── queue/             # Review queue with workers
│   │   └── circuitbreaker/    # Circuit breaker implementation
│   ├── app/                   # Application layer (HTTP handlers, validators)
│   ├── config/                # Configuration management
│   ├── domain/                # Business logic
│   │   ├── models/            # Domain models (PR, Review, etc.)
│   │   ├── ports/             # Interface definitions
│   │   ├── services/          # Core review service
│   │   └── errors/            # Domain errors
│   └── infrastructure/        # Cross-cutting concerns (rate limiting, retry)
├── profiles/                   # Review profile templates (markdown)
└── docs/                      # Documentation
```

## Documentation

- [Configuration Guide](docs/configuration.md)
- [Profiles Guide](docs/profiles.md)
- [Deployment Guide](docs/deployment.md)
- [Metrics Guide](docs/metrics.md)
- [Architecture Explained](docs/architecture-explained.md)

## License

MIT License - See LICENSE file for details

TODO: support multiple events.