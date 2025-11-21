# Templates Guide

Templates control how Claude reviews pull requests. This guide explains how to create, customize, and manage review templates.

## Overview

Templates are Markdown files containing instructions for Claude AI on how to review your code. Each template defines:

- The reviewer's role and expertise
- The review goals and focus areas
- The expected output format
- Project-specific guidelines and standards

## Template Structure

### Directory Layout

```
templates/
├── default/
│   └── prompt.md
├── strict-review/
│   └── prompt.md
├── backend-review/
│   └── prompt.md
└── frontend-review/
    └── prompt.md
```

Each template is a folder containing a `prompt.md` file. The folder name is used as the template identifier in configuration.

### Required File

Every template folder must contain:
- `prompt.md` - The template prompt with review instructions

### Template Anatomy

A template must contain these required sections:

```markdown
# Role:
[Define the role and expertise of the reviewer]

# Goal:
[Specify what the reviewer should accomplish]

# PR:
[Reference to PR information - uses template variables]
```

## Template Variables

Templates support variable substitution to inject PR-specific information:

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

### Example Usage

```markdown
# PR:
- **URL**: {{prUrl}}
- **Title**: {{title}}
- **Author**: {{author}}
- **Repository**: {{repository}}
- **Branch**: {{sourceBranch}} → {{destinationBranch}}

## Description:
{{description}}
```

## Default Template

The default template is located in `templates/default/prompt.md` and serves as the fallback when no specific template is configured.

### Example Default Template

```markdown
# Role:
You are an experienced software engineer conducting a thorough code review.

# Goal:
Review the pull request and provide constructive feedback on:
- Code quality and maintainability
- Potential bugs and edge cases
- Security vulnerabilities
- Performance concerns
- Best practices and design patterns

# PR:
- **URL**: {{prUrl}}
- **Title**: {{title}}
- **Author**: {{author}}
- **Repository**: {{repository}}
- **Branch**: {{sourceBranch}} → {{destinationBranch}}

## Description:
{{description}}

# Instructions:

1. Clone and examine the repository using the provided clone URL
2. Review all changed files in the source branch
3. Analyze code quality, security, and potential issues
4. Provide specific, actionable feedback with file names and line numbers
5. Suggest improvements where applicable

# Output Format:

Provide your review as a comment with:
- Summary of changes
- Issues found (if any) with severity: CRITICAL, HIGH, MEDIUM, LOW
- Suggestions for improvement
- LGTM status (Looks Good To Me - approve if no blocking issues)

At the end, include metrics in JSON format:

```json
{
  "isLgtm": true/false,
  "issueCount": number,
  "isReviewFailed": false,
  "failedReviewReason": ""
}
```
```

## Custom Templates

### Creating a New Template

1. Create a new folder in `templates/` directory:
   ```bash
   mkdir -p templates/security-review
   ```

2. Create `prompt.md` inside the folder:
   ```bash
   touch templates/security-review/prompt.md
   ```

3. Write your template with required sections

4. Configure the template in `config.yaml`

### Template Examples

#### Backend Review Template

```markdown
# Role:
You are a senior backend engineer specializing in API design, database optimization, and system architecture.

# Goal:
Review backend code changes focusing on:
- API design and RESTful principles
- Database queries and N+1 problems
- Error handling and logging
- Authentication and authorization
- Performance and scalability

# PR:
- **URL**: {{prUrl}}
- **Title**: {{title}}
- **Repository**: {{repository}}
- **Branch**: {{sourceBranch}} → {{destinationBranch}}

# Backend-Specific Checks:

1. **API Endpoints**
   - Proper HTTP methods and status codes
   - Request validation and error responses
   - API versioning compatibility

2. **Database**
   - Query efficiency and indexes
   - Transaction handling
   - Migration safety

3. **Security**
   - Input validation and sanitization
   - SQL injection prevention
   - Authentication/authorization checks

4. **Performance**
   - Database query optimization
   - Caching strategy
   - Resource usage

# Output Format:
[Standard review format with JSON metrics]
```

#### Frontend Review Template

```markdown
# Role:
You are a frontend specialist with expertise in React, accessibility, and performance optimization.

# Goal:
Review frontend code changes focusing on:
- Component design and reusability
- State management patterns
- Accessibility (a11y) compliance
- Performance optimization
- User experience

# PR:
- **URL**: {{prUrl}}
- **Title**: {{title}}
- **Repository**: {{repository}}

# Frontend-Specific Checks:

1. **React Components**
   - Proper hooks usage
   - Component composition
   - Props validation
   - Key props in lists

2. **Accessibility**
   - ARIA labels and roles
   - Keyboard navigation
   - Screen reader support
   - Color contrast

3. **Performance**
   - Unnecessary re-renders
   - Code splitting opportunities
   - Image optimization
   - Bundle size impact

4. **UX/UI**
   - Loading states
   - Error handling
   - Responsive design
   - User feedback

# Output Format:
[Standard review format with JSON metrics]
```

#### Security-Focused Template

```markdown
# Role:
You are a security engineer performing a security-focused code review.

# Goal:
Identify security vulnerabilities and compliance issues:
- OWASP Top 10 vulnerabilities
- Authentication and authorization flaws
- Data exposure and privacy concerns
- Input validation and sanitization
- Cryptography and secrets management

# PR:
- **URL**: {{prUrl}}
- **Title**: {{title}}

# Security Checks:

1. **Input Validation**
   - All user inputs validated
   - SQL injection prevention
   - XSS prevention
   - Command injection prevention

2. **Authentication/Authorization**
   - Proper authentication checks
   - Authorization enforcement
   - Session management
   - Token handling

3. **Data Protection**
   - Sensitive data encryption
   - Secure communication (HTTPS)
   - No secrets in code
   - PII handling compliance

4. **Dependencies**
   - Known vulnerable dependencies
   - Outdated packages
   - Supply chain risks

# Severity Levels:
- CRITICAL: Immediate security risk requiring urgent fix
- HIGH: Significant security concern
- MEDIUM: Security improvement recommended
- LOW: Minor security consideration

# Output Format:
[Standard review format with JSON metrics]
```

## Template Configuration

### Global Default

Set the default template for all repositories:

```yaml
templates:
  directory: ./templates
  default: default
```

### Project-Level Templates

Apply a template to all repositories in a project:

```yaml
templates:
  default: default
  projects:
    CI:
      template: strict-review
    BACKEND:
      template: backend-review
    FRONTEND:
      template: frontend-review
```

### Repository-Level Templates

Override templates for specific repositories:

```yaml
templates:
  default: default
  projects:
    CI:
      template: strict-review
      repositories:
        api-gateway: security-review
        user-service: backend-review
        admin-ui: frontend-review
```

### Template Resolution

The bot resolves templates in this priority order:

1. **Repository-specific**: `projects.<PROJECT>.repositories.<REPO>`
2. **Project-level**: `projects.<PROJECT>.template`
3. **Global default**: `default`

Example:
```yaml
templates:
  default: default  # 3. Fallback
  projects:
    CI:
      template: strict-review  # 2. Used for CI/other-repo
      repositories:
        api-gateway: security-review  # 1. Used for CI/api-gateway
```

Result:
- `CI/api-gateway` → uses `security-review`
- `CI/other-repo` → uses `strict-review`
- `OTHER/any-repo` → uses `default`

## Template Best Practices

### 1. Keep Templates Focused

Each template should have a clear, specific focus:
- Backend, frontend, infrastructure, security, etc.
- Don't try to cover everything in one template

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
- `Role:` - Defines reviewer expertise
- `Goal:` - States review objectives
- `PR:` - References PR information

### 4. Use Consistent Output Format

Require Claude to output metrics in JSON format:
```json
{
  "isLgtm": true,
  "issueCount": 2,
  "isReviewFailed": false,
  "failedReviewReason": ""
}
```

This enables metric tracking and monitoring.

### 5. Adjust Strictness by Environment

Use different templates based on environment:
- Development: Lenient, educational feedback
- Staging: Moderate, focused on quality
- Production: Strict, security and reliability focused

### 6. Document Template Purpose

Add a comment at the top of `prompt.md`:
```markdown
<!--
Template: Security Review
Purpose: Deep security analysis for critical infrastructure repositories
Target: INFRA project repositories handling sensitive data
Last Updated: 2024-01-15
-->
```

### 7. Validate Templates

Test new templates:
1. Create test PR in a repository
2. Configure repository to use new template
3. Trigger review and check output
4. Iterate based on results

## Template Metrics

The JSON output format enables metrics tracking:

```json
{
  "isLgtm": false,
  "issueCount": 5,
  "isReviewFailed": false,
  "failedReviewReason": ""
}
```

Fields:
- `isLgtm` (boolean) - Approval status
- `issueCount` (number) - Number of issues found
- `isReviewFailed` (boolean) - Whether review process failed
- `failedReviewReason` (string) - Reason for failure if applicable

These metrics are tracked per repository and exposed via Prometheus metrics endpoint.

## Template Validation

On startup, the application validates:
- Templates directory exists
- Default template folder exists
- `prompt.md` file exists in default template
- Template contains required sections

Missing templates result in warnings but don't prevent startup.

## Template Development Workflow

1. **Copy Default Template**
   ```bash
   cp -r templates/default templates/my-custom-template
   ```

2. **Edit Prompt**
   ```bash
   vim templates/my-custom-template/prompt.md
   ```

3. **Update Configuration**
   ```yaml
   templates:
     projects:
       MYPROJECT:
         template: my-custom-template
   ```

4. **Test on Sample PR**
   - Create or find test PR
   - Trigger review: `@bot /review`
   - Evaluate output quality

5. **Iterate and Refine**
   - Adjust instructions based on review quality
   - Test again until satisfied

6. **Document**
   - Add comment explaining template purpose
   - Update team documentation

## Troubleshooting Templates

### Template Not Found

Error: `failed to load template: no such file or directory`

Solutions:
- Verify template folder exists in `templates/` directory
- Check `prompt.md` exists inside template folder
- Verify `templates.directory` path in config

### Template Variables Not Substituted

If variables like `{{prUrl}}` appear literally in output:

- Verify variable names are spelled correctly
- Check for extra spaces: `{{ prUrl }}` won't work
- Variables are case-sensitive

### Poor Review Quality

If Claude's reviews aren't helpful:

- Make instructions more specific and actionable
- Add concrete examples of what to look for
- Adjust the role to match review focus
- Provide clearer output format requirements

### Timeout Issues

If reviews timeout frequently:

- Simplify template instructions
- Reduce scope of review
- Increase `claude.timeout_minutes` in config
- Split into multiple focused templates

## Advanced Template Techniques

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

## Next Steps

- Review [Configuration Guide](configuration.md) for template mapping
- See [Deployment Guide](deployment.md) for production setup
- Check example templates in `templates/` directory
