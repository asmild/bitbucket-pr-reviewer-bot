# Bitbucket Webhook Payload Examples

This directory contains example webhook payloads from both Bitbucket Data Center/Server and Bitbucket Cloud for testing and reference.

## Directory Structure

```
examples/
├── bitbucket-data-center/    # Bitbucket Data Center/Server webhook examples
│   ├── pr-opened.json
│   └── pr-comment-added.json
└── bitbucket-cloud/           # Bitbucket Cloud webhook examples
    ├── pr-opened.json
    └── pr-updated.json
```

## Bitbucket Data Center/Server

Examples for Bitbucket Data Center/Server API (formerly known as Bitbucket Server).

### bitbucket-data-center/pr-opened.json
Example payload for the `pr:opened` event, triggered when a new pull request is created.

**Key fields:**
- `eventKey`: "pr:opened"
- `pullRequest`: Contains PR details including title, description, author
- `pullRequest.fromRef`: Source branch information
- `pullRequest.toRef`: Destination branch information
- `repository`: Repository details with clone URLs and project information

### bitbucket-data-center/pr-comment-added.json
Example payload for the `pr:comment:added` event, triggered when a comment is added to a pull request.

**Key fields:**
- `eventKey`: "pr:comment:added"
- `comment`: Contains the comment details including ID, text, and author
- `comment.text`: The comment content (e.g., "@pr-reviewer-bot /review")
- `pullRequest`: Same structure as pr:opened event
- `repository`: Repository details

## Bitbucket Cloud

Examples for Bitbucket Cloud API (bitbucket.org).

### bitbucket-cloud/pr-opened.json
Example payload for the `pullrequest:created` event in Bitbucket Cloud.

**Key differences from Data Center:**
- Uses `pullrequest:created` instead of `pr:opened`
- Different JSON structure and field naming
- Uses workspace instead of project
- Different repository and user object structures

### bitbucket-cloud/pr-updated.json
Example payload for the `pullrequest:updated` event in Bitbucket Cloud.

**Key fields:**
- Event triggered when PR is updated (title, description, reviewers, etc.)
- Contains both old and new values for changed fields

## Usage

These examples can be used for:
1. **Testing webhook handlers** - Send these payloads to your webhook endpoint for testing
2. **Understanding payload structure** - Reference for implementing webhook parsing logic
3. **Documentation** - Examples of what data is available in each event type
4. **Development** - Test both Data Center and Cloud implementations

## Testing with curl

### Bitbucket Data Center

```bash
# Test PR opened event
curl -X POST http://localhost:8080/webhook/bitbucket \
  -H "Content-Type: application/json" \
  -H "X-Hub-Signature: sha256=your-signature" \
  -d @examples/bitbucket-data-center/pr-opened.json

# Test PR comment added event
curl -X POST http://localhost:8080/webhook/bitbucket \
  -H "Content-Type: application/json" \
  -H "X-Hub-Signature: sha256=your-signature" \
  -d @examples/bitbucket-data-center/pr-comment-added.json
```

### Bitbucket Cloud

```bash
# Test PR created event
curl -X POST http://localhost:8080/webhook/bitbucket-cloud \
  -H "Content-Type: application/json" \
  -H "X-Hook-UUID: your-webhook-uuid" \
  -d @examples/bitbucket-cloud/pr-opened.json

# Test PR updated event
curl -X POST http://localhost:8080/webhook/bitbucket-cloud \
  -H "Content-Type: application/json" \
  -H "X-Hook-UUID: your-webhook-uuid" \
  -d @examples/bitbucket-cloud/pr-updated.json
```

## Key Differences Between Data Center and Cloud

| Feature | Data Center/Server | Cloud |
|---------|-------------------|-------|
| Event naming | `pr:opened`, `pr:comment:added` | `pullrequest:created`, `pullrequest:updated` |
| Authentication | HMAC SHA256 signature | UUID-based webhook verification |
| Project structure | Projects contain repositories | Workspaces contain repositories |
| User format | `user.name`, `user.displayName` | `user.username`, `user.display_name` |
| Clone URLs | SSH and HTTP/HTTPS | Multiple clone links with names |

## Notes

- **Data Center examples** use `bitbucket.example.com` as a placeholder - replace with your actual Bitbucket server URL
- **Cloud examples** use real Bitbucket Cloud structure from `bitbucket.org`
- Field structure and naming conventions differ significantly between the two platforms
- For signature validation in Data Center, ensure your webhook secret is configured
- For Cloud webhooks, verify the `X-Hook-UUID` header matches your webhook configuration
- Currently, the application supports **Bitbucket Data Center only** - Cloud support is planned for future releases
