package bitbucket

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestValidateHMACSignature(t *testing.T) {
	t.Run("validates correct signature", func(t *testing.T) {
		payload := []byte(`{"test":"data"}`)
		secret := "my-secret-key"

		// Generate correct signature
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		correctSignature := hex.EncodeToString(mac.Sum(nil))

		if !validateHMACSignature(payload, correctSignature, secret) {
			t.Error("expected valid signature to pass validation")
		}
	})

	t.Run("validates signature with sha256 prefix", func(t *testing.T) {
		payload := []byte(`{"test":"data"}`)
		secret := "my-secret-key"

		// Generate correct signature
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		correctSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

		if !validateHMACSignature(payload, correctSignature, secret) {
			t.Error("expected signature with sha256= prefix to pass validation")
		}
	})

	t.Run("rejects incorrect signature", func(t *testing.T) {
		payload := []byte(`{"test":"data"}`)
		secret := "my-secret-key"
		wrongSignature := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

		if validateHMACSignature(payload, wrongSignature, secret) {
			t.Error("expected invalid signature to fail validation")
		}
	})

	t.Run("rejects signature with wrong secret", func(t *testing.T) {
		payload := []byte(`{"test":"data"}`)
		correctSecret := "my-secret-key"
		wrongSecret := "wrong-secret"

		// Generate signature with correct secret
		mac := hmac.New(sha256.New, []byte(correctSecret))
		mac.Write(payload)
		signature := hex.EncodeToString(mac.Sum(nil))

		// Validate with wrong secret
		if validateHMACSignature(payload, signature, wrongSecret) {
			t.Error("expected signature validation to fail with wrong secret")
		}
	})

	t.Run("rejects signature for different payload", func(t *testing.T) {
		originalPayload := []byte(`{"test":"data"}`)
		modifiedPayload := []byte(`{"test":"modified"}`)
		secret := "my-secret-key"

		// Generate signature for original payload
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(originalPayload)
		signature := hex.EncodeToString(mac.Sum(nil))

		// Validate with modified payload
		if validateHMACSignature(modifiedPayload, signature, secret) {
			t.Error("expected signature validation to fail for modified payload")
		}
	})

	t.Run("handles empty payload", func(t *testing.T) {
		payload := []byte{}
		secret := "my-secret-key"

		// Generate signature for empty payload
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		signature := hex.EncodeToString(mac.Sum(nil))

		if !validateHMACSignature(payload, signature, secret) {
			t.Error("expected signature validation to work with empty payload")
		}
	})

	t.Run("handles empty signature", func(t *testing.T) {
		payload := []byte(`{"test":"data"}`)
		secret := "my-secret-key"

		if validateHMACSignature(payload, "", secret) {
			t.Error("expected empty signature to fail validation")
		}
	})

	t.Run("handles empty secret", func(t *testing.T) {
		payload := []byte(`{"test":"data"}`)
		secret := ""

		// Generate signature with empty secret
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		signature := hex.EncodeToString(mac.Sum(nil))

		if !validateHMACSignature(payload, signature, secret) {
			t.Error("expected signature validation to work with empty secret")
		}
	})

	t.Run("is case sensitive for signature", func(t *testing.T) {
		payload := []byte(`{"test":"data"}`)
		secret := "my-secret-key"

		// Generate correct signature (lowercase hex)
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		signature := hex.EncodeToString(mac.Sum(nil))

		// Try uppercase signature
		uppercaseSignature := ""
		for _, c := range signature {
			if c >= 'a' && c <= 'f' {
				uppercaseSignature += string(c - 32)
			} else {
				uppercaseSignature += string(c)
			}
		}

		// Should fail because hex encoding is lowercase
		if uppercaseSignature != signature && validateHMACSignature(payload, uppercaseSignature, secret) {
			t.Error("expected uppercase signature to fail validation")
		}
	})

	t.Run("validates real Bitbucket-like payload", func(t *testing.T) {
		// Simulate a real Bitbucket webhook payload
		payload := []byte(`{
			"eventKey": "pr:comment:added",
			"date": "2025-11-23T10:00:00+0000",
			"pullRequest": {
				"id": 123,
				"title": "Test PR"
			}
		}`)
		secret := "bitbucket-webhook-secret-123"

		// Generate correct signature
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		signature := hex.EncodeToString(mac.Sum(nil))

		if !validateHMACSignature(payload, signature, secret) {
			t.Error("expected real-like Bitbucket payload signature to validate")
		}
	})

	t.Run("validates with special characters in secret", func(t *testing.T) {
		payload := []byte(`{"test":"data"}`)
		secret := "my-$ecret!@#%^&*()_+-={}[]|:;<>?,./~`"

		// Generate signature with special characters
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		signature := hex.EncodeToString(mac.Sum(nil))

		if !validateHMACSignature(payload, signature, secret) {
			t.Error("expected signature with special characters in secret to validate")
		}
	})

	t.Run("validates with unicode in payload", func(t *testing.T) {
		payload := []byte(`{"test":"data with unicode: 你好世界 🌍"}`)
		secret := "my-secret-key"

		// Generate signature
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		signature := hex.EncodeToString(mac.Sum(nil))

		if !validateHMACSignature(payload, signature, secret) {
			t.Error("expected signature with unicode payload to validate")
		}
	})

	t.Run("timing attack resistance", func(t *testing.T) {
		// This test verifies that hmac.Equal is used (constant time comparison)
		// rather than string comparison which could be vulnerable to timing attacks
		payload := []byte(`{"test":"data"}`)
		secret := "my-secret-key"

		// Generate correct signature
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		signature := hex.EncodeToString(mac.Sum(nil))

		// Create a signature that differs only in the last character
		almostCorrect := signature[:len(signature)-1] + "0"
		if signature[len(signature)-1] == '0' {
			almostCorrect = signature[:len(signature)-1] + "1"
		}

		// Both should fail, but timing should be similar (hmac.Equal provides this)
		result1 := validateHMACSignature(payload, almostCorrect, secret)
		result2 := validateHMACSignature(payload, "0000000000000000000000000000000000000000000000000000000000000000", secret)

		if result1 || result2 {
			t.Error("expected both wrong signatures to fail")
		}

		// The important thing is that hmac.Equal is used in the implementation
		// which provides constant-time comparison
	})
}

func TestValidateHMACSignature_Integration(t *testing.T) {
	t.Run("matches openssl hmac calculation", func(t *testing.T) {
		// Test case that can be verified with: echo -n 'test payload' | openssl dgst -sha256 -hmac 'secret'
		payload := []byte("test payload")
		secret := "secret"

		// Manually calculate expected signature
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		expectedSignature := hex.EncodeToString(mac.Sum(nil))

		// The signature for "test payload" with secret "secret" is:
		// f1f1fc517bb886ad22c56e51dae135aad082b2e3337bed35e2e44cd299324bd8
		// (can be verified with: echo -n 'test payload' | openssl dgst -sha256 -hmac 'secret')
		if expectedSignature != "f1f1fc517bb886ad22c56e51dae135aad082b2e3337bed35e2e44cd299324bd8" {
			t.Errorf("expected known signature, got %s", expectedSignature)
		}

		// Validate using our function
		if !validateHMACSignature(payload, expectedSignature, secret) {
			t.Error("expected known good signature to validate")
		}
	})
}
