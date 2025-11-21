package bitbucket

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/logger"
)

// doRequest performs an HTTP request with authentication and standard headers
func (c *Client) doRequest(method, url string, body []byte) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewBuffer(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication
	req.SetBasicAuth(c.user, c.token)

	// Set standard headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	return resp, nil
}

// get performs a GET request
func (c *Client) get(url string) ([]byte, error) {
	logger.Debugf("GET %s", url)

	resp, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return c.handleResponse(resp)
}

// post performs a POST request
func (c *Client) post(url string, body []byte) ([]byte, error) {
	logger.Debugf("POST %s", url)

	resp, err := c.doRequest("POST", url, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return c.handleResponse(resp)
}

// put performs a PUT request
func (c *Client) put(url string, body []byte) ([]byte, error) {
	logger.Debugf("PUT %s", url)

	resp, err := c.doRequest("PUT", url, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return c.handleResponse(resp)
}

// delete performs a DELETE request
func (c *Client) delete(url string) error {
	logger.Debugf("DELETE %s", url)

	resp, err := c.doRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// For DELETE, we don't care about response body
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// handleResponse processes the HTTP response and returns body or error
func (c *Client) handleResponse(resp *http.Response) ([]byte, error) {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return bodyBytes, nil
}
