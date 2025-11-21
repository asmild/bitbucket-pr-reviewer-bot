# Bitbucket PR Reviewer Bot

An intelligent automated pull request reviewer that integrates Bitbucket Data Center with Claude AI to provide comprehensive code reviews.

## Features

- Automated PR reviews powered by Claude AI
- Dual trigger modes: automatic on PR open or manual via comments
- Customizable review templates per project/repository
- Circuit breaker pattern for fault tolerance
- Prometheus metrics and monitoring
- Webhook signature validation for security
- Docker and Kubernetes deployment support

## Quick Start

### Prerequisites

- Go 1.25+
- Claude CLI installed and configured
- Bitbucket Data Center instance
- Bitbucket app password or personal access token

### Installation

#### Option 1: Download Pre-built Binary (Recommended)

Download the latest binary for your platform from [GitHub Releases](https://github.com/asmild/bitbucket-pr-reviewer-bot/releases):

```bash
# Linux AMD64
wget https://github.com/asmild/bitbucket-pr-reviewer-bot/releases/latest/download/bb-pr-reviewer-linux-amd64
chmod +x bb-pr-reviewer-linux-amd64
./bb-pr-reviewer-linux-amd64

# macOS ARM64 (Apple Silicon)
wget https://github.com/asmild/bitbucket-pr-reviewer-bot/releases/latest/download/bb-pr-reviewer-darwin-arm64
chmod +x bb-pr-reviewer-darwin-arm64
./bb-pr-reviewer-darwin-arm64

# macOS AMD64 (Intel)
wget https://github.com/asmild/bitbucket-pr-reviewer-bot/releases/latest/download/bb-pr-reviewer-darwin-amd64
chmod +x bb-pr-reviewer-darwin-amd64
./bb-pr-reviewer-darwin-amd64

# Windows
# Download bb-pr-reviewer-windows.exe from the releases page
```

#### Option 2: Build from Source

1. Clone the repository:
```bash
git clone https://github.com/asmild/bitbucket-pr-reviewer-bot.git
cd bitbucket-pr-reviewer-bot
```

2. Build using Makefile:
```bash
make build-local
./bb-pr-reviewer
```

Or build manually:
```bash
go build -o bb-pr-reviewer ./cmd/server/main.go
./bb-pr-reviewer
```

#### Configuration

Create configuration file:
```bash
cp config.example.yaml config.yaml
```

Edit `config.yaml` with your Bitbucket credentials:
```yaml
bitbucket:
  user: your-bot-username
  token: your-app-password
  webhook_secret: your-webhook-secret
  allowed_project_keys:
    - PROJ1
    - PROJ2
  event_type: "comment_added"
  trigger_keyword: "/review"
```

### Docker Deployment

```bash
docker build -t pr-reviewer:latest .

docker run -d \
  --name pr-reviewer \
  -p 8080:8080 \
  -e BITBUCKET_USER=your-username \
  -e BITBUCKET_TOKEN=your-token \
  -v $(pwd)/config.yaml:/app/config.yaml \
  -v $(pwd)/templates:/app/templates \
  pr-reviewer:latest
```

### Bitbucket Webhook Setup

1. Navigate to Repository/Project Settings → Webhooks
2. Create webhook with:
   - URL: `https://your-server.com/webhook/bitbucket/pr`
   - Secret: Your configured webhook secret
   - Events:
     - `Pull request opened` (for automatic reviews)
     - `Pull request comment added` (for manual reviews)

## Usage

### Automatic Review Mode

Configure `event_type: "pr_opened"` in your config. PRs will be automatically reviewed when opened.

### Manual Review Mode (Comment-Triggered)

Configure `event_type: "comment_added"` in your config. Trigger reviews by mentioning the bot:

```
@pr-review-bot /review
```

The bot will:
1. Acknowledge with an eyes reaction
2. Clone the repository and checkout the source branch
3. Analyze the code changes using Claude AI
4. Post a detailed review comment with findings

## Configuration

The bot supports configuration via YAML file and environment variables (env vars override YAML).

### Minimal Configuration

```yaml
bitbucket:
  user: bot-username
  token: app-password
```

### Environment Variables

```bash
export BITBUCKET_USER=bot-username
export BITBUCKET_TOKEN=your-token
export BITBUCKET_WEBHOOK_SECRET=your-secret
export PORT=8080
```

For detailed configuration options, see [Configuration Guide](docs/configuration.md).

## Templates

Templates control how Claude reviews your code. Create custom templates for different projects or repositories.

### Template Structure

```
templates/
├── default/
│   └── prompt.md
├── backend-review/
│   └── prompt.md
└── security-review/
    └── prompt.md
```

### Configure Per-Project Templates

```yaml
templates:
  directory: ./templates
  default: default
  projects:
    BACKEND:
      template: backend-review
      repositories:
        api-gateway: security-review
    FRONTEND:
      template: frontend-review
```

For detailed template documentation, see [Templates Guide](docs/templates.md).

## API Endpoints

### Health Check

```
GET /health
```

Returns service status.

### Metrics

```
GET /metrics
```

Returns Prometheus metrics including:
- Total PRs reviewed
- Success/failure rates
- Review duration
- Issues found
- LGTM count

### Webhook

```
POST /webhook/bitbucket/pr
```

Receives Bitbucket webhook events for PR reviews.

## Monitoring

The bot exposes Prometheus metrics at `/metrics`:

```
# Review metrics
pr_reviews_success_total{repository="my-repo"} 45
pr_reviews_failed_total{repository="my-repo",error_type="timeout"} 2
pr_review_duration_seconds{repository="my-repo",status="success",quantile="0.5"} 12.5

# Issue tracking
pr_lgtm_total{repository="my-repo"} 30
pr_issues_found_total{repository="my-repo"} 120
```

## Architecture

```
├── cmd/server/              # Application entry point
├── internal/
│   ├── bitbucket/          # Bitbucket API client
│   ├── claude/             # Claude AI integration
│   ├── config/             # Configuration management
│   ├── circuitbreaker/     # Circuit breaker pattern
│   ├── git/                # Git operations
│   ├── http/               # HTTP utilities
│   ├── logger/             # Structured logging
│   ├── metrics/            # Prometheus metrics
│   ├── queue/              # PR processing queue
│   ├── startup/            # Startup dependency validation
│   └── templates/          # Template management
├── pkg/models/             # Data models
├── templates/              # Review templates
└── docs/                   # Documentation
```

## Documentation

- [Configuration Guide](docs/configuration.md) - Detailed configuration options
- [Templates Guide](docs/templates.md) - Creating and managing review templates
- [Deployment Guide](docs/deployment.md) - Production deployment instructions

## Development

### Running Tests

```bash
go test ./...
```

### Code Coverage

```bash
go test -cover ./...
```

### Building

Build optimized cross-platform binaries:
```bash
make build
```

Or build for local development:
```bash
make build-local
```

## Deployment Options

The bot supports multiple deployment options:

- **Systemd Service** - Linux service management
- **Docker Container** - Containerized deployment
- **Docker Compose** - Multi-container orchestration
- **Kubernetes** - Cloud-native deployment

See [Deployment Guide](docs/deployment.md) for detailed instructions.

## Metrics and Observability

### Available Metrics

- `pr_reviews_created_total` - Total review requests
- `pr_reviews_success_total` - Successful reviews
- `pr_reviews_failed_total` - Failed reviews with error types
- `pr_review_duration_seconds` - Review duration histogram
- `pr_lgtm_total` - LGTMs issued
- `pr_issues_found_total` - Issues identified

### Logging

Structured JSON logs with configurable levels:
- `debug` - Detailed diagnostic information
- `info` - General operational messages
- `warn` - Warning conditions
- `error` - Error conditions

Logs are written to:
- Console (stdout) - Real-time monitoring
- Files (`./logs/`) - Persistent storage with rotation

## Security

- **Webhook Signature Validation** - HMAC-SHA256 verification
- **Project Filtering** - Restrict to specific projects
- **Credential Management** - Environment variable support
- **HTTPS Support** - TLS/SSL encryption recommended

## Performance

- **Low Memory Footprint** - ~20-40 MB idle
- **Fast Startup** - <1 second
- **Concurrent Processing** - Goroutine-based queue
- **Circuit Breaker** - Fault tolerance for transient failures

## Troubleshooting

### Common Issues

**Service won't start:**
- Check `bitbucket.user` and `bitbucket.token` are set
- Verify port is not in use
- Check logs for configuration errors

**Webhooks not received:**
- Verify webhook URL is accessible
- Check firewall rules
- Validate webhook secret matches

**Reviews timeout:**
- Increase `claude.timeout_minutes`
- Simplify template instructions
- Check Claude API rate limits

See [Deployment Guide](docs/deployment.md) for detailed troubleshooting.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Submit a pull request

## Acknowledgments

This project was inspired by [bitbucket-automatic-pr-reviewer](https://github.com/TinTinWinata/bitbucket-automatic-pr-reviewer) and adapted for Bitbucket Data Center with a complete rewrite in Go, adding features like:
- Native Bitbucket Data Center support
- Startup dependency validation
- Multi-location configuration
- Custom template system per project/repository

## License

MIT License - See LICENSE file for details

## Support

- GitHub Issues: Report bugs and request features
- Documentation: See `docs/` directory
- Logs: Check application logs for errors
