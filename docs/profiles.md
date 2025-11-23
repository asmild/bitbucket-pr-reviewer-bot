# Profiles Guide

Profiles control how Claude reviews pull requests. This guide explains how to create, customize, and manage review profiles.

## Overview

Profiles are Markdown files containing instructions for Claude AI on how to review your code. Each profile defines:

- The reviewer's role and expertise
- The review goals and focus areas
- The expected output format
- Project-specific guidelines and standards

## Profile Structure

### Directory Layout

```
profiles/
├── default.md
├── security-focused.md
├── performance-review.md
└── quick-review.md
```

Each profile is a single `.md` file. The filename (without `.md` extension) is used as the profile identifier in configuration.

### Required Sections

A profile must contain these required sections:

```markdown
**Role:**
[Define the role and expertise of the reviewer]

**Goal:**
[Specify what the reviewer should accomplish]

**PR:**
[Reference to PR information - uses template variables]
```

**Important:** The format uses `**Role:**`, `**Goal:**`, and `**PR:**` (bold markdown), not `# Role:` (headers).

### File Format

- Profiles are plain Markdown (`.md`) files
- Must be at least 100 characters long
- Must contain required sections: Role, Goal, PR
- Stored directly in `profiles/` directory (not in subdirectories)

## Profile Variables

Profiles support variable substitution to inject PR-specific information:

| Variable | Description | Example |
|----------|-------------|---------|
| `{{prUrl}}` | Full URL to the pull request | `https://bitbucket.example.com/projects/CI/repos/api/pull-requests/123` |
| `{{title}}` | PR title | `Add user authentication` |
| `{{description}}` | PR description | `This PR implements JWT-based authentication...` |
| `{{author}}` | PR author username | `john.doe` |
| `{{repository}}` | Repository slug | `api-service` |
| `{{sourceBranch}}` | Source branch name | `feature/auth` |
| `{{destinationBranch}}` | Destination branch name | `main` |
| `{{repoCloneUrl}}` | Repository clone URL | `https://bitbucket.example.com/scm/ci/api.git` |
| `{{projectKey}}` | Project key | `CI` |
| `{{prId}}` | Pull request ID | `123` |

### Example Usage

```markdown
**PR:**
`{{prUrl}}`

- **Title**: {{title}}
- **Author**: {{author}}
- **Repository**: {{repository}}
- **Branch**: {{sourceBranch}} → {{destinationBranch}}

## Description:
{{description}}
```

## Default Profile

The default profile (`profiles/default.md`) serves as the fallback when no specific profile is configured.

### Example Default Profile

```markdown
**Role:**
You are an experienced software engineer conducting a thorough code review.

**Goal:**
Review the pull request and provide constructive feedback on code quality, potential bugs, security vulnerabilities, and best practices.

**PR:**
`{{prUrl}}`

---

## Review Instructions

1. Analyze all changed files
2. Identify logic errors, security concerns, performance issues
3. Check for missing edge case handling and tests
4. Provide specific, actionable feedback

## Output Format

Provide your review as a single comment with:
- Summary of changes (1-2 sentences)
- Issues found with severity and specific file/line references
- Suggestions for improvement

## Metrics

After posting the review, output metrics in JSON format:

\```json
{
  "isLgtm": true/false,
  "issueCount": <number>,
  "isReviewFailed": false,
  "failedReviewReason": null
}
\```
```

## Custom Profiles

### Creating a New Profile

1. Create a new file in `profiles/` directory:
   ```bash
   touch profiles/security-review.md
   ```

2. Write your profile with required sections (Role, Goal, PR)

3. Configure the profile in `config.yaml`

### Profile Examples

#### Security-Focused Profile

```markdown
**Role:**
You are a security engineer performing a security-focused code review.

**Goal:**
Identify security vulnerabilities and compliance issues:
- OWASP Top 10 vulnerabilities
- Authentication and authorization flaws
- Data exposure and privacy concerns
- Input validation and sanitization
- Cryptography and secrets management

**PR:**
`{{prUrl}}`

---

## Security Checks

### 1. Input Validation
- All user inputs validated
- SQL injection prevention
- XSS prevention
- Command injection prevention

### 2. Authentication/Authorization
- Proper authentication checks
- Authorization enforcement
- Session management
- Token handling

### 3. Data Protection
- Sensitive data encryption
- Secure communication (HTTPS)
- No secrets in code
- PII handling compliance

### 4. Dependencies
- Known vulnerable dependencies
- Outdated packages
- Supply chain risks

## Severity Levels
- CRITICAL: Immediate security risk requiring urgent fix
- HIGH: Significant security concern
- MEDIUM: Security improvement recommended
- LOW: Minor security consideration

## Output Metrics
[Standard JSON metrics format]
```

#### Performance Review Profile

```markdown
**Role:**
You are a performance optimization specialist reviewing code for efficiency and scalability.

**Goal:**
Identify performance bottlenecks and optimization opportunities:
- Database query efficiency
- Algorithm complexity
- Memory usage
- Caching opportunities
- Resource management

**PR:**
`{{prUrl}}`

---

## Performance Checks

### 1. Database Queries
- Query efficiency and indexes
- N+1 query problems
- Missing pagination
- Query result caching

### 2. Algorithm Complexity
- Time complexity (O(n), O(n²), etc.)
- Space complexity
- Unnecessary loops
- Better data structures

### 3. Resource Management
- Connection pooling
- File handle cleanup
- Memory leaks
- Thread safety

### 4. Caching
- Cache hit rate opportunities
- Cache invalidation strategy
- Appropriate cache TTL

## Output Format
[Standard review format with JSON metrics]
```

#### Quick Review Profile

```markdown
**Role:**
You are conducting a fast, focused code review for small changes.

**Goal:**
Quickly identify critical issues only - no minor style feedback. Focus on:
- Obvious bugs
- Security vulnerabilities
- Breaking changes

**PR:**
`{{prUrl}}`

---

## Quick Check Items

1. **Critical bugs** - Logic errors, null pointer issues
2. **Security** - Authentication bypasses, injection vulnerabilities
3. **Breaking changes** - API compatibility, data migration needs

Skip:
- Code style
- Minor optimizations
- Documentation improvements

## Output
Brief summary with only critical/high severity issues.

## Metrics
[Standard JSON metrics format]
```

## Profile Configuration

### Global Default

Set the default profile for all repositories:

```yaml
profiles:
  directory: ./profiles
  default: default
```

### Project-Level Profiles

Apply a profile to all repositories in a project:

```yaml
profiles:
  directory: ./profiles
  default: default
  projects:
    CI:
      profile: security-focused
    BACKEND:
      profile: performance-review
    FRONTEND:
      profile: quick-review
```

### Repository-Level Profiles

Override profiles for specific repositories:

```yaml
profiles:
  directory: ./profiles
  default: default
  projects:
    CI:
      profile: security-focused
      repositories:
        api-gateway: security-focused
        user-service: performance-review
        admin-ui: quick-review
```

### Profile Resolution

The bot resolves profiles in this priority order:

1. **Repository-specific**: `projects.<PROJECT>.repositories.<REPO>`
2. **Project-level**: `projects.<PROJECT>.profile`
3. **Global default**: `default`

Example:
```yaml
profiles:
  default: default  # 3. Fallback
  projects:
    CI:
      profile: security-focused  # 2. Used for CI/other-repo
      repositories:
        api-gateway: performance-review  # 1. Used for CI/api-gateway
```

Result:
- `CI/api-gateway` → uses `performance-review`
- `CI/other-repo` → uses `security-focused`
- `OTHER/any-repo` → uses `default`

## Profile Best Practices

### 1. Keep Profiles Focused

Each profile should have a clear, specific focus:
- Security, performance, quick-review, etc.
- Don't try to cover everything in one profile

### 2. Be Specific and Actionable

Good:
```markdown
Check for SQL injection vulnerabilities:
- Verify all database queries use parameterized statements
- Ensure user input is never concatenated into SQL strings
```

Bad:
```markdown
Check for security issues.
```

### 3. Include Required Sections

Always include:
- `**Role:**` - Defines reviewer expertise
- `**Goal:**` - States review objectives
- `**PR:**` - References PR information (use `{{prUrl}}` variable)

### 4. Use Consistent Output Format

Always require Claude to output metrics in JSON format:
```json
{
  "isLgtm": true/false,
  "issueCount": <number>,
  "isReviewFailed": false,
  "failedReviewReason": null
}
```

This enables metric tracking and monitoring.

### 5. Adjust Strictness by Environment

Use different profiles based on environment:
- Development: Lenient, educational feedback
- Staging: Moderate, focused on quality
- Production: Strict, security and reliability focused

### 6. Document Profile Purpose

Add a comment at the top of the profile:
```markdown
<!--
Profile: Security Review
Purpose: Deep security analysis for critical infrastructure repositories
Target: INFRA project repositories handling sensitive data
Last Updated: 2024-01-15
-->
```

### 7. Test Profiles

Test new profiles:
1. Create test PR in a repository
2. Configure repository to use new profile
3. Trigger review and check output
4. Iterate based on results

## Profile Validation

On startup, the application validates:
- Profiles directory exists
- Default profile file exists (`default.md`)
- Profile contains required sections (Role, Goal, PR)
- Profile meets minimum length (100 characters)

Missing or invalid profiles result in errors and prevent startup.

## Profile Development Workflow

1. **Copy Default Profile**
   ```bash
   cp profiles/default.md profiles/my-custom-profile.md
   ```

2. **Edit Profile**
   ```bash
   vim profiles/my-custom-profile.md
   ```

3. **Update Configuration**
   ```yaml
   profiles:
     projects:
       MYPROJECT:
         profile: my-custom-profile
   ```

4. **Test on Sample PR**
   - Create or find test PR
   - Trigger review: `@bot /review`
   - Evaluate output quality

5. **Iterate and Refine**
   - Adjust instructions based on review quality
   - Test again until satisfied

6. **Document**
   - Add comment explaining profile purpose
   - Update team documentation

## Troubleshooting Profiles

### Profile Not Found

Error: `failed to load profile: no such file or directory`

Solutions:
- Verify profile file exists in `profiles/` directory
- Check filename is correct (case-sensitive)
- Verify `profiles.directory` path in config
- Ensure file has `.md` extension

### Profile Variables Not Substituted

If variables like `{{prUrl}}` appear literally in output:

- Verify variable names are spelled correctly
- Check for extra spaces: `{{ prUrl }}` won't work
- Variables are case-sensitive: `{{prurl}}` won't work

### Poor Review Quality

If Claude's reviews aren't helpful:

- Make instructions more specific and actionable
- Add concrete examples of what to look for
- Adjust the role to match review focus
- Provide clearer output format requirements
- Check if profile is too long or complex

### Timeout Issues

If reviews timeout frequently:

- Simplify profile instructions
- Reduce scope of review (e.g., focus on critical issues only)
- Increase `claude.timeout_minutes` in config
- Use faster Claude model (haiku instead of sonnet)

### Validation Errors

If profile fails validation on startup:

```
Profile validation failed: missing required section 'Role'
```

Solutions:
- Ensure `**Role:**` section exists (bold format)
- Verify `**Goal:**` section exists
- Check `**PR:**` section exists
- Ensure profile is at least 100 characters
- Check for typos in section headers

## Advanced Profile Techniques

### Conditional Instructions

```markdown
If the PR modifies database migrations:
- Verify migration is reversible
- Check for data loss risks
- Ensure proper indexing

If the PR modifies API endpoints:
- Verify backward compatibility
- Check authentication requirements
- Validate input/output schemas
```

### Severity Guidance

```markdown
Rate each issue by severity:
- CRITICAL: Security vulnerability, data loss risk, system crash
- HIGH: Significant bug, major performance impact
- MEDIUM: Code quality issue, minor bug
- LOW: Style improvement, minor optimization
```

### Context-Aware Reviews

```markdown
For files in `src/api/`:
- Focus on REST principles and error handling

For files in `src/database/`:
- Focus on query optimization and migrations

For files in `test/`:
- Focus on test coverage and assertions
```

### Language-Specific Checks

```markdown
For Go code:
- Check for goroutine leaks
- Verify proper error handling (not ignoring errors)
- Check for race conditions

For Python code:
- Check for proper exception handling
- Verify type hints are used
- Look for security issues (SQL injection, etc.)
```

## Profile Variables Reference

Complete list of available variables:

```markdown
- {{prUrl}} - https://bitbucket.example.com/projects/CI/repos/api/pull-requests/123
- {{prId}} - 123
- {{title}} - Add user authentication
- {{description}} - This PR implements JWT-based authentication...
- {{author}} - john.doe
- {{repository}} - api-service
- {{projectKey}} - CI
- {{sourceBranch}} - feature/auth
- {{destinationBranch}} - main
- {{repoCloneUrl}} - https://bitbucket.example.com/scm/ci/api.git
```

All variables are substituted before the profile is sent to Claude.

## Next Steps

- Review [Configuration Guide](configuration.md) for profile mapping
- See [Deployment Guide](deployment.md) for production setup
- Check example profiles in `profiles/` directory
- Create custom profiles for your projects
