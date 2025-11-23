# Deployment Guide

This guide walks you through deploying the PR Reviewer bot to production.

> **Note:** This application uses a modern **Hexagonal Architecture** with clean separation of concerns, dependency injection, and production-grade resilience patterns including circuit breakers, retry logic, and comprehensive startup validation.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Bitbucket Setup](#bitbucket-setup)
- [Application Setup](#application-setup)
- [Deployment Options](#deployment-options)
- [Post-Deployment](#post-deployment)
- [Monitoring](#monitoring)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### Required Software

- **Go 1.21+** - For building the application
- **Claude CLI** - Installed and configured with API key
- **Git** - For cloning repositories during review
- **Bitbucket Data Center** - Target Bitbucket instance

### Required Accounts

- **Bitbucket User Account** - Dedicated bot account with appropriate permissions
- **Claude API Access** - Valid API key for Claude AI
- **Server/VM** - For hosting the application

### Required Permissions

Bitbucket bot account needs:
- Read access to target repositories
- Write access to post comments on pull requests
- Ability to add reactions to comments

## Bitbucket Setup

### 1. Create Bot User Account

1. Log into Bitbucket Data Center as admin
2. Navigate to **Administration** → **Users**
3. Click **Create User**
4. Fill in details:
   - Username: `pr-review-bot` (or your preference)
   - Display Name: `PR Review Bot`
   - Email: Bot email address
   - Password: Strong password

### 2. Generate App Password

1. Log in as the bot user
2. Navigate to **Manage Account** → **HTTP access tokens** or **App passwords**
3. Click **Create token/password**
4. Name: `pr-reviewer-service`
5. Permissions: Select at minimum:
   - Repository: Read, Write
   - Pull requests: Read, Write
6. Save the generated token securely

### 3. Grant Repository Access

Grant bot user access to target repositories:

**Option A: Project-level access**
1. Navigate to **Project Settings** → **Permissions**
2. Add bot user with **Read** or **Write** permission

**Option B: Repository-level access**
1. Navigate to **Repository Settings** → **Permissions**
2. Add bot user with **Read** or **Write** permission

### 4. Configure Webhook

For each repository or project:

1. Navigate to **Repository/Project Settings** → **Webhooks**
2. Click **Create webhook**
3. Configure webhook:
   - **Name**: `PR Review Bot`
   - **URL**: `https://your-server.com/webhook/bitbucket/pr`
   - **Status**: Active
   - **SSL/TLS**: Verify (if using HTTPS)
   - **Secret**: Generate strong secret (save for configuration)
   - **Events**:
     - For automatic reviews: Select `Pull request opened`
     - For manual reviews: Select `Comment added`
4. Click **Create**

## Application Setup

### 1. Clone Repository

```bash
git clone https://github.com/yourusername/pr-reviewer.git
cd pr-reviewer
```

### 2. Build Application

```bash
go build -o pr-reviewer ./cmd/server/main.go
```

This creates the `pr-reviewer` binary.

### 3. Install Claude CLI

```bash
# Install Claude CLI (follow official Claude CLI installation guide)
# Example for macOS/Linux:
curl -sSL https://claude.ai/cli/install.sh | bash

# Verify installation
claude --version

# Authenticate with API key
claude auth login
```

### 4. Create Configuration

Copy the example configuration:

```bash
cp config.example.yaml config.yaml
```

Edit `config.yaml`:

```yaml
server:
  port: 8080

claude:
  model: sonnet
  timeout_minutes: 10

bitbucket:
  user: "pr-review-bot"
  token: "your-app-password-from-step-2"
  webhook_secret: "your-webhook-secret-from-step-4"
  allowed_project_keys:
    - CI
    - BACKEND
  event_type: "comment_added"
  trigger_keyword: "/review"

profiles:
  directory: ./profiles
  default: default

circuit_breaker:
  failure_threshold: 3
  reset_timeout_ms: 30000

metrics:
  persistence:
    enabled: true
    type: filesystem
    path: ./metrics-storage

logging:
  level: info
  file_retention_days: 30
  enable_console: true
  enable_file: true
```

### 5. Prepare Profiles

Ensure profiles directory exists:

```bash
mkdir -p profiles
```

Create default profile if not exists:

```bash
cat > profiles/default.md << 'EOF'
**Role:**
You are an experienced software engineer conducting a code review.

**Goal:**
Review the pull request thoroughly and provide constructive feedback.

**PR:**
`{{prUrl}}`

- **Title**: {{title}}
- **Author**: {{author}}

Review the changes and provide feedback.
EOF
```

### 6. Set Environment Variables

For production, use environment variables for secrets:

```bash
export BITBUCKET_USER="pr-review-bot"
export BITBUCKET_TOKEN="your-app-password"
export BITBUCKET_WEBHOOK_SECRET="your-webhook-secret"
```

### 7. Verify Startup Dependencies

The application performs comprehensive startup validation before running. You can test this:

```bash
./pr-reviewer
```

The application will check:
- ✓ Configuration values (Bitbucket credentials, ports, timeouts)
- ✓ Claude CLI installation and authentication
- ✓ Git installation
- ✓ Profiles directory and default profile existence
- ✓ Directory write permissions (git base dir, logs)

**Example successful output:**
```
✓ Claude CLI: claude-cli v1.2.3
✓ Claude CLI authentication verified
✓ git version 2.39.0
✓ Profiles: Found default profile at ./profiles/default.md
✓ Git base directory: ./projects (writable)
✓ Logs directory: ./logs (writable)
All startup dependency checks passed

Application started successfully - ready to process webhooks
```

**If validation fails**, the application will exit with clear error messages showing exactly what needs to be fixed.

## Deployment Options

### Option 1: Systemd Service (Linux)

Create systemd service file:

```bash
sudo vim /etc/systemd/system/pr-reviewer.service
```

```ini
[Unit]
Description=PR Reviewer Bot
After=network.target

[Service]
Type=simple
User=pr-reviewer
WorkingDirectory=/opt/pr-reviewer
ExecStart=/opt/pr-reviewer/pr-reviewer
Restart=on-failure
RestartSec=10

# Environment variables
Environment="BITBUCKET_USER=pr-review-bot"
Environment="BITBUCKET_TOKEN=your-token"
Environment="BITBUCKET_WEBHOOK_SECRET=your-secret"
Environment="LOG_LEVEL=info"

# Limits
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable pr-reviewer
sudo systemctl start pr-reviewer
sudo systemctl status pr-reviewer
```

### Option 2: Docker Container

Create `Dockerfile`:

```dockerfile
FROM golang:1.21 as builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o pr-reviewer ./cmd/server/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates git curl

# Install Claude CLI
RUN curl -sSL https://claude.ai/cli/install.sh | sh

WORKDIR /app

COPY --from=builder /app/pr-reviewer .
COPY --from=builder /app/profiles ./profiles
COPY --from=builder /app/config.example.yaml ./config.yaml

EXPOSE 8080

CMD ["./pr-reviewer"]
```

Build and run:

```bash
docker build -t pr-reviewer:latest .

docker run -d \
  --name pr-reviewer \
  -p 8080:8080 \
  -e BITBUCKET_USER=pr-review-bot \
  -e BITBUCKET_TOKEN=your-token \
  -e BITBUCKET_WEBHOOK_SECRET=your-secret \
  -v $(pwd)/config.yaml:/app/config.yaml \
  -v $(pwd)/profiles:/app/profiles \
  -v $(pwd)/logs:/app/logs \
  pr-reviewer:latest
```

### Option 3: Docker Compose

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  pr-reviewer:
    build: .
    container_name: pr-reviewer
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - BITBUCKET_USER=pr-review-bot
      - BITBUCKET_TOKEN=${BITBUCKET_TOKEN}
      - BITBUCKET_WEBHOOK_SECRET=${BITBUCKET_WEBHOOK_SECRET}
      - LOG_LEVEL=info
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - ./profiles:/app/profiles:ro
      - ./logs:/app/logs
      - ./metrics-storage:/app/metrics-storage
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

Create `.env` file:

```bash
BITBUCKET_TOKEN=your-app-password
BITBUCKET_WEBHOOK_SECRET=your-webhook-secret
```

Start:

```bash
docker-compose up -d
docker-compose logs -f
```

### Option 4: Kubernetes Deployment

Create `k8s-deployment.yaml`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: pr-reviewer-secrets
type: Opaque
stringData:
  bitbucket-token: your-app-password
  webhook-secret: your-webhook-secret
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: pr-reviewer-config
data:
  config.yaml: |
    server:
      port: 8080
    bitbucket:
      user: pr-review-bot
      event_type: comment_added
      trigger_keyword: /review
    profiles:
      directory: ./profiles
      default: default
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pr-reviewer
spec:
  replicas: 1
  selector:
    matchLabels:
      app: pr-reviewer
  template:
    metadata:
      labels:
        app: pr-reviewer
    spec:
      containers:
      - name: pr-reviewer
        image: pr-reviewer:latest
        ports:
        - containerPort: 8080
        env:
        - name: BITBUCKET_USER
          value: pr-review-bot
        - name: BITBUCKET_TOKEN
          valueFrom:
            secretKeyRef:
              name: pr-reviewer-secrets
              key: bitbucket-token
        - name: BITBUCKET_WEBHOOK_SECRET
          valueFrom:
            secretKeyRef:
              name: pr-reviewer-secrets
              key: webhook-secret
        volumeMounts:
        - name: config
          mountPath: /app/config.yaml
          subPath: config.yaml
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: config
        configMap:
          name: pr-reviewer-config
---
apiVersion: v1
kind: Service
metadata:
  name: pr-reviewer
spec:
  selector:
    app: pr-reviewer
  ports:
  - port: 80
    targetPort: 8080
  type: LoadBalancer
```

Deploy:

```bash
kubectl apply -f k8s-deployment.yaml
kubectl get pods
kubectl logs -f deployment/pr-reviewer
```

## Post-Deployment

### 1. Verify Service is Running

```bash
curl http://your-server:8080/health
```

Expected response:
```json
{"status":"ok","message":"PR Automation service is running"}
```

### 2. Check Metrics

```bash
curl http://your-server:8080/metrics
```

Should return Prometheus metrics.

### 3. Test Webhook

Create a test PR and:
- For `pr_opened`: PR should be automatically reviewed
- For `comment_added`: Add comment with trigger keyword (e.g., `@pr-review-bot /review`)

### 4. Monitor Logs

```bash
# Systemd
sudo journalctl -u pr-reviewer -f

# Docker
docker logs -f pr-reviewer

# Docker Compose
docker-compose logs -f

# Kubernetes
kubectl logs -f deployment/pr-reviewer
```

### 5. Configure Reverse Proxy (Optional)

For HTTPS and domain name:

**Nginx:**

```nginx
server {
    listen 443 ssl http2;
    server_name pr-reviewer.yourdomain.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

**Caddy:**

```
pr-reviewer.yourdomain.com {
    reverse_proxy localhost:8080
}
```

## Monitoring

### Health Check Endpoint

- **URL**: `/health`
- **Method**: GET
- **Response**: `{"status":"ok","message":"PR Automation service is running"}`

Monitor with:
```bash
*/5 * * * * curl -f http://your-server:8080/health || alert
```

### Metrics Endpoint

- **URL**: `/metrics`
- **Method**: GET
- **Format**: Prometheus metrics

Integrate with Prometheus:

```yaml
scrape_configs:
  - job_name: 'pr-reviewer'
    static_configs:
      - targets: ['your-server:8080']
```

### Available Metrics

- `pr_reviewer_webhook_received_total` - Total webhooks received by event type
- `pr_reviewer_review_started_total` - Reviews started by project
- `pr_reviewer_review_completed_total` - Completed reviews by project and status
- `pr_reviewer_review_failed_total` - Failed reviews by project and error type
- `pr_reviewer_review_duration_seconds` - Review duration histogram
- `pr_reviewer_queue_size` - Current queue size
- `pr_reviewer_git_clone_duration_seconds` - Git operation duration
- `pr_reviewer_circuit_breaker_state` - Circuit breaker status

**For comprehensive metrics documentation including:**
- Detailed metric descriptions
- Histogram bucket interpretation
- PromQL query examples
- Grafana dashboard setup
- Alerting rules

See [Metrics Guide](metrics.md).

### Log Monitoring

Logs are written to:
- Console (stdout) if `enable_console: true`
- Files in `./logs/` if `enable_file: true`

Log levels: `debug`, `info`, `warn`, `error`

Monitor for:
- `level=error` - Application errors
- `Webhook signature validation failed` - Invalid webhooks
- `Circuit breaker opened` - Service degradation
- `Claude CLI execution failed` - Review failures

## Backup and Recovery

### Configuration Backup

```bash
cp config.yaml config.yaml.backup
```

### Metrics Backup

If using filesystem persistence:

```bash
tar -czf metrics-backup-$(date +%Y%m%d).tar.gz metrics-storage/
```

### Profiles Backup

```bash
tar -czf profiles-backup-$(date +%Y%m%d).tar.gz profiles/
```

## Security Best Practices

1. **Use HTTPS** - Always use HTTPS for webhook endpoint
2. **Validate Webhooks** - Set `webhook_secret` and verify signatures
3. **Restrict Access** - Use firewall rules to limit access
4. **Rotate Credentials** - Periodically rotate Bitbucket tokens
5. **Limit Project Access** - Use `allowed_project_keys` to restrict scope
6. **Monitor Logs** - Watch for suspicious activity
7. **Keep Updated** - Regularly update dependencies and Claude CLI

## Scaling Considerations

### Single Instance Limitations

Current implementation uses in-memory queue:
- **Queue State**: Lost on restart
- **Concurrency**: Processes one PR at a time
- **High Availability**: No built-in redundancy

### Scaling Options

For high-volume deployments:

1. **External Queue** - Redis, RabbitMQ, or AWS SQS
2. **Multiple Workers** - Horizontal scaling with shared queue
3. **Database State** - PostgreSQL or similar for persistence
4. **Load Balancer** - Distribute webhook load across instances

## Troubleshooting

### Service Won't Start

Check logs for:
- Configuration validation errors
- Missing required fields (user, token)
- Port already in use
- Profiles directory not found

### Webhooks Not Received

Verify:
- Webhook URL is correct and accessible from Bitbucket
- Firewall allows inbound connections
- Service is listening on configured port
- Webhook is active in Bitbucket settings

### Reviews Not Posting

Check:
- Bot user has write permissions on repository
- Bitbucket token is valid and not expired
- API calls succeeding (check logs for errors)
- Circuit breaker not open

### Claude Timeouts

Solutions:
- Increase `claude.timeout_minutes`
- Simplify template instructions
- Use faster model (haiku instead of opus)
- Check Claude API rate limits

### High Memory Usage

- Reduce concurrent reviews
- Monitor for memory leaks
- Restart service periodically
- Increase server resources

## Maintenance

### Regular Tasks

**Daily:**
- Monitor error logs
- Check metrics for anomalies

**Weekly:**
- Review failed reviews
- Check disk space for logs
- Verify bot user access

**Monthly:**
- Rotate Bitbucket tokens
- Update dependencies
- Review and optimize profiles
- Backup configuration and metrics

### Updates

To update the application:

1. Backup configuration and data
2. Pull latest code or download new binary
3. Review changelog for breaking changes
4. Update configuration if needed
5. Rebuild application
6. Restart service
7. Verify functionality

## Support and Resources

- **Configuration Guide**: [configuration.md](configuration.md)
- **Profiles Guide**: [profiles.md](profiles.md)
- **Metrics Guide**: [metrics.md](metrics.md)
- **GitHub Issues**: Report bugs and request features
- **Logs**: Check application logs for detailed errors

## Next Steps

After deployment:
1. Test with sample PRs
2. Gather feedback from team
3. Refine profiles based on results
4. Set up monitoring and alerts
5. Document team-specific workflows
