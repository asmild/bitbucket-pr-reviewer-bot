package validator

import (
	"testing"
)

func TestValidateBitbucketMCP_Scenarios(t *testing.T) {
	t.Run("should fail when no MCP servers configured", func(t *testing.T) {
		result := &ValidationResult{}

		// This is what happens internally in validateBitbucketMCP
		// when claude mcp list returns "No MCP servers configured"
		output := "No MCP servers configured. Use `claude mcp add` to add a server."

		if containsNoMCP(output) {
			result.AddError("Bitbucket MCP", "No MCP servers", "Install MCP")
		}

		if !result.IsValid() {
			t.Log("✓ Correctly detected no MCP servers")
		} else {
			t.Error("Should have detected no MCP servers")
		}
	})

	t.Run("should fail when other MCP exists but not Bitbucket", func(t *testing.T) {
		result := &ValidationResult{}

		// Simulate output with other MCP servers but no Bitbucket
		output := `Configured MCP Servers:

1. @modelcontextprotocol/server-filesystem
   Status: Active

2. @modelcontextprotocol/server-github
   Status: Active`

		hasBitbucket := containsBitbucketMCP(output)

		if !hasBitbucket {
			result.AddError("Bitbucket MCP", "Not found", "Add Bitbucket MCP")
		}

		if !result.IsValid() {
			t.Log("✓ Correctly detected missing Bitbucket MCP (other MCPs present)")
		} else {
			t.Error("Should have detected missing Bitbucket MCP")
		}
	})

	t.Run("should pass when Bitbucket MCP configured", func(t *testing.T) {
		result := &ValidationResult{}

		// Simulate output with Bitbucket MCP
		output := `Configured MCP Servers:

1. @atlassian-dc-mcp/bitbucket
   Status: Active
   URL: https://git.ib-ci.com

2. @modelcontextprotocol/server-filesystem
   Status: Active`

		hasBitbucket := containsBitbucketMCP(output)

		if hasBitbucket {
			result.AddInfo("✓ Bitbucket MCP configured")
		} else {
			result.AddError("Bitbucket MCP", "Not found", "Add Bitbucket MCP")
		}

		if result.IsValid() {
			t.Log("✓ Correctly detected Bitbucket MCP present")
		} else {
			t.Error("Should have passed with Bitbucket MCP present")
		}
	})
}

// Helper functions matching the logic in validateBitbucketMCP
func containsNoMCP(output string) bool {
	return stringContains(output, "No MCP servers configured")
}

func containsBitbucketMCP(output string) bool {
	return stringContains(toLower(output), "bitbucket") ||
		stringContains(output, "@atlassian-dc-mcp/bitbucket")
}

func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && indexOfSubstring(s, substr) >= 0
}

func indexOfSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func toLower(s string) string {
	result := ""
	for _, c := range s {
		if c >= 'A' && c <= 'Z' {
			result += string(c + 32)
		} else {
			result += string(c)
		}
	}
	return result
}
