# Metrics Guide

This guide explains the monitoring and metrics system for the PR Reviewer bot.

## Overview

The PR Reviewer bot exposes Prometheus-compatible metrics at `/metrics` endpoint. These metrics provide visibility into webhook processing, review execution, queue status, and system health.

## Accessing Metrics

### Metrics Endpoint

**URL:** `http://your-server:8080/metrics`
**Method:** GET
**Format:** Prometheus text format

```bash
curl http://localhost:8080/metrics
```

### Prometheus Integration

Add to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'pr-reviewer'
    static_configs:
      - targets: ['your-server:8080']
    scrape_interval: 15s
```

## Available Metrics

### Webhook Metrics

#### `pr_reviewer_webhook_received_total`

**Type:** Counter
**Labels:** `event_type`
**Description:** Total number of webhooks received by event type

**Example:**
```
pr_reviewer_webhook_received_total{event_type="pr:comment:added"} 45
pr_reviewer_webhook_received_total{event_type="pr:opened"} 12
pr_reviewer_webhook_received_total{event_type="rate_limited"} 3
```

**Use cases:**
- Monitor webhook traffic volume
- Identify rate limiting issues
- Track different event types

---

### Review Execution Metrics

#### `pr_reviewer_review_started_total`

**Type:** Counter
**Labels:** `project`
**Description:** Total number of reviews started

**Example:**
```
pr_reviewer_review_started_total{project="CI"} 34
pr_reviewer_review_started_total{project="INFRA"} 12
```

**Use cases:**
- Track review volume per project
- Identify most active projects
- Monitor bot usage

---

#### `pr_reviewer_review_completed_total`

**Type:** Counter
**Labels:** `project`, `status`
**Description:** Total number of reviews completed

**Example:**
```
pr_reviewer_review_completed_total{project="CI",status="lgtm"} 28
pr_reviewer_review_completed_total{project="CI",status="issues_found"} 6
pr_reviewer_review_completed_total{project="INFRA",status="lgtm"} 10
```

**Status values:**
- `lgtm` - Review approved (Looks Good To Me)
- `issues_found` - Review completed but found issues
- `failed` - Review process failed

**Use cases:**
- Track review outcomes
- Calculate approval rates
- Monitor review quality

---

#### `pr_reviewer_review_failed_total`

**Type:** Counter
**Labels:** `project`, `error_type`
**Description:** Total number of failed reviews by error type

**Example:**
```
pr_reviewer_review_failed_total{project="CI",error_type="timeout"} 2
pr_reviewer_review_failed_total{project="CI",error_type="git_clone"} 1
pr_reviewer_review_failed_total{project="INFRA",error_type="claude_api"} 3
```

**Error types:**
- `timeout` - Review exceeded timeout
- `git_clone` - Failed to clone repository
- `claude_api` - Claude API error
- `network` - Network connectivity issues
- `unknown` - Unclassified errors

**Use cases:**
- Identify failure patterns
- Debug infrastructure issues
- Alert on error rate increases

---

#### `pr_reviewer_review_duration_seconds`

**Type:** Histogram
**Labels:** `project`
**Description:** Duration of reviews in seconds
**Buckets:** `10, 30, 60, 120, 180, 300, 600` (10s, 30s, 1m, 2m, 3m, 5m, 10m)

**Example output:**
```
pr_reviewer_review_duration_seconds_bucket{project="CI",le="10"} 2
pr_reviewer_review_duration_seconds_bucket{project="CI",le="30"} 5
pr_reviewer_review_duration_seconds_bucket{project="CI",le="60"} 8
pr_reviewer_review_duration_seconds_bucket{project="CI",le="120"} 15
pr_reviewer_review_duration_seconds_bucket{project="CI",le="180"} 20
pr_reviewer_review_duration_seconds_bucket{project="CI",le="300"} 22
pr_reviewer_review_duration_seconds_bucket{project="CI",le="600"} 23
pr_reviewer_review_duration_seconds_bucket{project="CI",le="+Inf"} 23
pr_reviewer_review_duration_seconds_sum{project="CI"} 2456.8
pr_reviewer_review_duration_seconds_count{project="CI"} 23
```

**Understanding Histogram Buckets:**

Histograms track the **distribution** of values over predefined ranges (buckets). Each bucket shows how many observations fell **at or below** that threshold.

- `le="10"` → 2 reviews completed in ≤10 seconds
- `le="30"` → 5 reviews completed in ≤30 seconds (cumulative)
- `le="60"` → 8 reviews completed in ≤60 seconds (cumulative)
- `le="120"` → 15 reviews completed in ≤2 minutes (cumulative)
- `le="+Inf"` → Total count of all observations

**Key fields:**
- `_bucket{le="X"}` - Cumulative count of reviews ≤ X seconds
- `_sum` - Total time spent on all reviews (in seconds)
- `_count` - Total number of reviews

**Calculate average duration:**
```
pr_reviewer_review_duration_seconds_sum / pr_reviewer_review_duration_seconds_count
= 2456.8 / 23
= 106.8 seconds (≈1.8 minutes average)
```

**Why buckets show 0 initially:**

If you see `0` in all buckets right after a review, it's because Prometheus histograms are **cumulative counters**. The actual duration is in the `_sum` field:

```
# After one review that took 198.6 seconds:
pr_reviewer_review_duration_seconds_bucket{project="CI",le="10"} 0      # No reviews ≤10s
pr_reviewer_review_duration_seconds_bucket{project="CI",le="30"} 0      # No reviews ≤30s
pr_reviewer_review_duration_seconds_bucket{project="CI",le="60"} 0      # No reviews ≤60s
pr_reviewer_review_duration_seconds_bucket{project="CI",le="120"} 0     # No reviews ≤2m
pr_reviewer_review_duration_seconds_bucket{project="CI",le="180"} 0     # No reviews ≤3m
pr_reviewer_review_duration_seconds_bucket{project="CI",le="300"} 1     # 1 review ≤5m ✓
pr_reviewer_review_duration_seconds_bucket{project="CI",le="600"} 1     # 1 review ≤10m ✓
pr_reviewer_review_duration_seconds_bucket{project="CI",le="+Inf"} 1    # Total: 1 review
pr_reviewer_review_duration_seconds_sum{project="CI"} 198.6             # Actual duration!
pr_reviewer_review_duration_seconds_count{project="CI"} 1               # Total count
```

The review took **198.6 seconds** (found in `_sum`), which falls in the 180-300 second range.

**Use cases:**
- Calculate average review time
- Identify slow reviews (p95, p99 percentiles)
- Detect performance degradation
- Set realistic timeout values

**PromQL Examples:**

```promql
# Average review duration
rate(pr_reviewer_review_duration_seconds_sum[5m]) / rate(pr_reviewer_review_duration_seconds_count[5m])

# 95th percentile review duration
histogram_quantile(0.95, rate(pr_reviewer_review_duration_seconds_bucket[5m]))

# Reviews completing within 2 minutes
sum(rate(pr_reviewer_review_duration_seconds_bucket{le="120"}[5m]))
```

---

### Queue Metrics

#### `pr_reviewer_queue_size`

**Type:** Gauge
**Description:** Current number of items in the review queue

**Example:**
```
pr_reviewer_queue_size 3
```

**Use cases:**
- Monitor queue backlog
- Detect processing delays
- Trigger scaling decisions
- Alert on queue buildup

**Typical values:**
- `0` - Queue is empty (all reviews processed)
- `1-5` - Normal processing load
- `>10` - Queue building up, reviews delayed
- `>50` - System overloaded or stuck

---

### Git Operation Metrics

#### `pr_reviewer_git_clone_duration_seconds`

**Type:** Histogram
**Description:** Duration of git clone/update operations
**Buckets:** `1, 5, 10, 30, 60, 120` (1s, 5s, 10s, 30s, 1m, 2m)

**Example:**
```
pr_reviewer_git_clone_duration_seconds_bucket{le="1"} 12
pr_reviewer_git_clone_duration_seconds_bucket{le="5"} 34
pr_reviewer_git_clone_duration_seconds_bucket{le="10"} 40
pr_reviewer_git_clone_duration_seconds_bucket{le="+Inf"} 42
pr_reviewer_git_clone_duration_seconds_sum 187.3
pr_reviewer_git_clone_duration_seconds_count 42
```

**Use cases:**
- Identify slow repository clones
- Detect network issues
- Optimize git operations

---

### Circuit Breaker Metrics

#### `pr_reviewer_circuit_breaker_state`

**Type:** Gauge
**Labels:** `name`
**Description:** Circuit breaker state

**Values:**
- `0` - Closed (normal operation)
- `1` - Half-open (testing recovery)
- `2` - Open (failing fast)

**Example:**
```
pr_reviewer_circuit_breaker_state{name="main"} 0
```

**Use cases:**
- Monitor system health
- Alert on circuit breaker opening
- Track recovery attempts

---

#### `pr_reviewer_circuit_breaker_transitions_total`

**Type:** Counter
**Labels:** `from`, `to`
**Description:** Total number of circuit breaker state transitions

**Example:**
```
pr_reviewer_circuit_breaker_transitions_total{from="closed",to="open"} 3
pr_reviewer_circuit_breaker_transitions_total{from="open",to="half_open"} 3
pr_reviewer_circuit_breaker_transitions_total{from="half_open",to="closed"} 2
pr_reviewer_circuit_breaker_transitions_total{from="half_open",to="open"} 1
```

**Use cases:**
- Track circuit breaker activity
- Identify unstable periods
- Measure recovery success rate

---

### Cost & Usage Metrics

These metrics track Claude API usage and costs. They are persisted across restarts when persistence is enabled.

#### `pr_reviewer_tokens_input_total`

**Type:** Counter
**Labels:** `project`, `model`
**Description:** Total number of input tokens used for Claude reviews

**Example:**
```
pr_reviewer_tokens_input_total{project="CI",model="sonnet"} 1250000
pr_reviewer_tokens_input_total{project="INFRA",model="sonnet"} 450000
```

**Use cases:**
- Track token consumption per project
- Identify high-usage projects
- Forecast API costs

---

#### `pr_reviewer_tokens_output_total`

**Type:** Counter
**Labels:** `project`, `model`
**Description:** Total number of output tokens generated by Claude reviews

**Example:**
```
pr_reviewer_tokens_output_total{project="CI",model="sonnet"} 85000
pr_reviewer_tokens_output_total{project="INFRA",model="sonnet"} 32000
```

**Use cases:**
- Track response token usage
- Monitor review verbosity
- Cost analysis (output tokens typically cost more)

---

#### `pr_reviewer_cost_usd_total`

**Type:** Counter
**Labels:** `project`, `model`
**Description:** Total cost in USD for Claude reviews

**Example:**
```
pr_reviewer_cost_usd_total{project="CI",model="sonnet"} 45.75
pr_reviewer_cost_usd_total{project="INFRA",model="sonnet"} 18.20
```

**Use cases:**
- Track spending per project
- Budget monitoring and alerts
- Chargeback to teams
- ROI analysis

**PromQL Examples:**

```promql
# Total cost across all projects
sum(pr_reviewer_cost_usd_total)

# Cost per project (last 24h increase)
sum by (project) (increase(pr_reviewer_cost_usd_total[24h]))

# Cost rate per hour
sum(rate(pr_reviewer_cost_usd_total[1h])) * 3600
```

---

### Review Quality Metrics

These metrics track the quality and findings of code reviews.

#### `pr_reviewer_critical_issues_total`

**Type:** Counter
**Labels:** `project`
**Description:** Total number of critical issues found in reviews

**Example:**
```
pr_reviewer_critical_issues_total{project="CI"} 23
pr_reviewer_critical_issues_total{project="INFRA"} 8
```

---

#### `pr_reviewer_warning_issues_total`

**Type:** Counter
**Labels:** `project`
**Description:** Total number of warning-level issues found in reviews

**Example:**
```
pr_reviewer_warning_issues_total{project="CI"} 156
pr_reviewer_warning_issues_total{project="INFRA"} 42
```

---

#### `pr_reviewer_suggestions_total`

**Type:** Counter
**Labels:** `project`
**Description:** Total number of suggestions/improvements found in reviews

**Example:**
```
pr_reviewer_suggestions_total{project="CI"} 312
pr_reviewer_suggestions_total{project="INFRA"} 89
```

**Use cases for quality metrics:**
- Track code quality trends
- Identify projects needing attention
- Measure review effectiveness

**PromQL Examples:**

```promql
# Critical issues rate per review
sum by (project) (rate(pr_reviewer_critical_issues_total[1h])) / sum by (project) (rate(pr_reviewer_review_completed_total[1h]))

# Total issues found (all severities)
sum by (project) (pr_reviewer_critical_issues_total + pr_reviewer_warning_issues_total + pr_reviewer_suggestions_total)
```

---

#### `pr_reviewer_unique_pr_reviewed_total`

**Type:** Counter
**Labels:** `project`, `repo`
**Description:** Total number of unique PRs reviewed (each PR counted once regardless of retries)

**Example:**
```
pr_reviewer_unique_pr_reviewed_total{project="CI",repo="backend"} 145
pr_reviewer_unique_pr_reviewed_total{project="CI",repo="frontend"} 89
```

**Use cases:**
- Track actual PR coverage
- Compare with `review_started_total` to identify retry rates

---

## Metrics Persistence

### Overview

Cost and usage metrics can be persisted to disk and restored on application restart. This ensures cumulative counters (tokens, costs, issues) survive restarts.

**Persisted metrics:**
- `pr_reviewer_tokens_input_total`
- `pr_reviewer_tokens_output_total`
- `pr_reviewer_cost_usd_total`
- `pr_reviewer_critical_issues_total`
- `pr_reviewer_warning_issues_total`
- `pr_reviewer_suggestions_total`

### Configuration

```yaml
metrics:
  persistence:
    enabled: true
    type: filesystem  # or "sqlite"
    path: ./metrics-storage
    save_interval_ms: 30000  # Save every 30 seconds
```

### Storage Types

**Filesystem (JSON)**
- Stores metrics in `metrics.json` file
- Human-readable format
- Atomic writes (temp file + rename)
- Good for simple deployments

```yaml
metrics:
  persistence:
    enabled: true
    type: filesystem
    path: ./metrics-storage
```

**SQLite**
- Stores metrics in `metrics.db` database
- Better for concurrent access
- Supports upsert operations
- Good for production deployments

```yaml
metrics:
  persistence:
    enabled: true
    type: sqlite
    path: ./metrics-storage
```

### How It Works

1. **On startup**: Metrics are restored from storage and added to Prometheus counters
2. **Periodically**: Metrics are saved at `save_interval_ms` intervals
3. **On shutdown**: Final save is performed before exit

### Environment Variables

```bash
export METRICS_PERSISTENCE_ENABLED=true
export METRICS_PERSISTENCE_TYPE=filesystem  # or "sqlite"
export METRICS_PERSISTENCE_PATH=/var/lib/pr-reviewer/metrics
export METRICS_PERSISTENCE_SAVE_INTERVAL_MS=30000
```

### Disabling Persistence

If persistence is not configured or `enabled: false`, the application uses a no-op persister that discards data. This is safe and won't cause errors.

```yaml
# Persistence disabled (default)
metrics:
  persistence:
    enabled: false
```

## Monitoring Setup

### Grafana Dashboard

Create a Grafana dashboard with these panels:

**1. Review Activity Panel**
```promql
# Reviews per hour
sum(rate(pr_reviewer_review_completed_total[1h])) by (project)
```

**2. Review Duration Panel**
```promql
# Average review time (5-minute window)
rate(pr_reviewer_review_duration_seconds_sum[5m]) / rate(pr_reviewer_review_duration_seconds_count[5m])
```

**3. Success Rate Panel**
```promql
# Review success rate
sum(rate(pr_reviewer_review_completed_total{status="lgtm"}[5m])) / sum(rate(pr_reviewer_review_completed_total[5m])) * 100
```

**4. Queue Size Panel**
```promql
# Current queue size
pr_reviewer_queue_size
```

**5. Error Rate Panel**
```promql
# Errors per minute
sum(rate(pr_reviewer_review_failed_total[1m])) by (error_type)
```

**6. Cost Tracking Panel**
```promql
# Total cost by project
sum by (project) (pr_reviewer_cost_usd_total)

# Daily cost rate
sum(increase(pr_reviewer_cost_usd_total[24h]))
```

**7. Token Usage Panel**
```promql
# Input vs output tokens by project
sum by (project) (pr_reviewer_tokens_input_total)
sum by (project) (pr_reviewer_tokens_output_total)
```

### Alerting Rules

**High Error Rate Alert**
```yaml
- alert: HighReviewErrorRate
  expr: rate(pr_reviewer_review_failed_total[5m]) > 0.1
  for: 5m
  annotations:
    summary: "High review error rate detected"
    description: "Error rate is {{ $value }} errors/sec"
```

**Queue Buildup Alert**
```yaml
- alert: ReviewQueueBacklog
  expr: pr_reviewer_queue_size > 10
  for: 10m
  annotations:
    summary: "Review queue backlog detected"
    description: "Queue size is {{ $value }} items"
```

**Circuit Breaker Open Alert**
```yaml
- alert: CircuitBreakerOpen
  expr: pr_reviewer_circuit_breaker_state > 1
  for: 1m
  annotations:
    summary: "Circuit breaker is open"
    description: "Service is failing fast due to repeated errors"
```

**Slow Review Alert**
```yaml
- alert: SlowReviews
  expr: rate(pr_reviewer_review_duration_seconds_sum[10m]) / rate(pr_reviewer_review_duration_seconds_count[10m]) > 300
  for: 10m
  annotations:
    summary: "Reviews are taking too long"
    description: "Average review time is {{ $value }}s (>5 minutes)"
```

**Daily Cost Budget Alert**
```yaml
- alert: DailyCostBudgetExceeded
  expr: sum(increase(pr_reviewer_cost_usd_total[24h])) > 100
  for: 5m
  annotations:
    summary: "Daily cost budget exceeded"
    description: "Spent ${{ $value }} in the last 24 hours (budget: $100)"
```

**High Cost Project Alert**
```yaml
- alert: HighCostProject
  expr: sum by (project) (increase(pr_reviewer_cost_usd_total[24h])) > 50
  for: 5m
  annotations:
    summary: "Project {{ $labels.project }} has high costs"
    description: "Project spent ${{ $value }} in the last 24 hours"
```

## Troubleshooting

### Metrics Not Showing Up

**Check metrics endpoint is accessible:**
```bash
curl http://localhost:8080/metrics
```

**Verify Prometheus is scraping:**
```bash
# Check Prometheus targets page
http://your-prometheus:9090/targets
```

**Check application logs:**
```bash
journalctl -u pr-reviewer | grep metrics
```

---

### Histogram Buckets All Zero

This is **normal** if:
- Only one review has been completed
- Review duration falls outside smaller buckets
- Check `_sum` and `_count` fields for actual values

**Example:**
```
# This is CORRECT for a 198-second review:
_bucket{le="10"} 0    # Review didn't finish in 10s
_bucket{le="180"} 0   # Review didn't finish in 3m
_bucket{le="300"} 1   # Review finished within 5m ✓
_sum 198.6            # Actual duration: 198.6 seconds
_count 1              # Total reviews: 1
```

**Calculate average:** `198.6 / 1 = 198.6 seconds`

---

### Metrics Reset on Restart

Prometheus metrics are **stored in memory** and reset on application restart. This is expected behavior.

For persistent metrics across restarts:
- Use Prometheus long-term storage
- Enable metrics persistence (limited functionality)
- Use external time-series database

---

### High Memory Usage

Prometheus metrics consume memory based on:
- Number of unique label combinations
- Histogram bucket count

**Solutions:**
- Reduce label cardinality (avoid high-cardinality labels like PR IDs)
- Adjust histogram buckets if needed
- Monitor memory usage with: `go_memstats_alloc_bytes`

---

## Best Practices

### 1. Use Appropriate Time Windows

```promql
# Good - 5-minute rate for dashboards
rate(pr_reviewer_review_completed_total[5m])

# Bad - 10-second rate (too noisy)
rate(pr_reviewer_review_completed_total[10s])
```

### 2. Calculate Rates for Counters

Always use `rate()` or `increase()` with counters:

```promql
# Good - shows reviews per second
rate(pr_reviewer_review_completed_total[5m])

# Bad - shows total count (not useful for trends)
pr_reviewer_review_completed_total
```

### 3. Use Histogram Quantiles

```promql
# 95th percentile review duration
histogram_quantile(0.95, rate(pr_reviewer_review_duration_seconds_bucket[5m]))

# 99th percentile review duration
histogram_quantile(0.99, rate(pr_reviewer_review_duration_seconds_bucket[5m]))
```

### 4. Monitor Error Rates

```promql
# Error rate as percentage of total reviews
sum(rate(pr_reviewer_review_failed_total[5m])) / sum(rate(pr_reviewer_review_started_total[5m])) * 100
```

### 5. Set Up Alerts

Don't just collect metrics - set up alerts for:
- High error rates (>5%)
- Queue backlog (>10 items for >10 minutes)
- Circuit breaker open
- Slow reviews (p95 >5 minutes)
- No reviews in 1 hour (if expecting activity)

## Example Queries

### Review Volume by Project

```promql
sum by (project) (rate(pr_reviewer_review_completed_total[1h]))
```

### Success Rate

```promql
sum(rate(pr_reviewer_review_completed_total{status="lgtm"}[5m])) / sum(rate(pr_reviewer_review_completed_total[5m])) * 100
```

### Average Review Time (Last Hour)

```promql
rate(pr_reviewer_review_duration_seconds_sum[1h]) / rate(pr_reviewer_review_duration_seconds_count[1h])
```

### Reviews Per Day

```promql
increase(pr_reviewer_review_completed_total[24h])
```

### Error Breakdown

```promql
sum by (error_type) (rate(pr_reviewer_review_failed_total[5m]))
```

### Git Clone Performance

```promql
histogram_quantile(0.95, rate(pr_reviewer_git_clone_duration_seconds_bucket[5m]))
```

## Next Steps

- Set up Prometheus scraping
- Create Grafana dashboard
- Configure alerting rules
- Monitor metrics during testing
- Adjust thresholds based on your environment
