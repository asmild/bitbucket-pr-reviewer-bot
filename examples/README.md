# Bitbucket Webhook Payload Examples

This directory contains example webhook payloads from Bitbucket Data Center/Server for testing and reference.

## Files

### pr-opened.json
Example payload for the `pr:opened` event, triggered when a new pull request is created.

**Key fields:**
- `eventKey`: "pr:opened"
- `pullRequest`: Contains PR details including title, description, author
- `pullRequest.fromRef`: Source branch information
- `pullRequest.toRef`: Destination branch information
- `repository`: Repository details with clone URLs and project information

### pr-comment-added.json
Example payload for the `pr:comment:added` event, triggered when a comment is added to a pull request.

**Key fields:**
- `eventKey`: "pr:comment:added"
- `comment`: Contains the comment details including ID, text, and author
- `comment.text`: The comment content (e.g., "@pr-reviewer-bot /review")
- `pullRequest`: Same structure as pr:opened event
- `repository`: Repository details

## Usage

These examples can be used for:
1. **Testing webhook handlers** - Send these payloads to your webhook endpoint for testing
2. **Understanding payload structure** - Reference for implementing webhook parsing logic
3. **Documentation** - Examples of what data is available in each event type

## Testing with curl

```bash
# Test PR opened event
curl -X POST http://localhost:8080/webhook/bitbucket/pr \
  -H "Content-Type: application/json" \
  -H "X-Hub-Signature: sha256=your-signature" \
  -d @examples/bitbucket-payloads/pr-opened.json

# Test PR comment added event
curl -X POST http://localhost:8080/webhook/bitbucket/pr \
  -H "Content-Type: application/json" \
  -H "X-Hub-Signature: sha256=your-signature" \
  -d @examples/bitbucket-payloads/pr-comment-added.json
```

## Notes

- These payloads are for **Bitbucket Data Center/Server**, not Bitbucket Cloud
- The examples use `bitbucket.example.com` as a placeholder - replace with your actual Bitbucket server URL
- Field structure and naming conventions are specific to Data Center/Server API
- For signature validation, ensure your webhook secret is configured in your application
