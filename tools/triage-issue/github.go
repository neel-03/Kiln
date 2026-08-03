// Package main implements the issue-triage tool. This file holds the
// GitHub REST API client.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// githubAPIBase is a var, not a const, so tests can redirect it to a mock
// server instead of hitting the real GitHub API.
var githubAPIBase = "https://api.github.com"

// requireEnv reads a required environment variable, exiting the process
// with a clear message if it's unset -- matching the original script's
// requireEnv(), which also exits rather than panicking or returning an
// error, since a missing required env var means there's nothing sensible
// left to do.
func requireEnv(name string) string {
	v := os.Getenv(name)
	if v == "" {
		fmt.Fprintf(os.Stderr, "Missing required env var: %s\n", name)
		os.Exit(1)
	}
	return v
}

// githubClient is a small wrapper around net/http bound to one token,
// equivalent to the closure returned by makeGitHubClient(token) in the
// original JS.
type githubClient struct {
	token      string
	httpClient *http.Client
}

func newGitHubClient(token string) *githubClient {
	return &githubClient{
		token:      token,
		httpClient: &http.Client{},
	}
}

// do issues a request against the GitHub REST API. If body is non-nil it's
// JSON-encoded as the request payload. If out is non-nil, a successful
// response body is JSON-decoded into it -- callers pass nil for endpoints
// whose response they don't need (mirroring the original returning `null`
// for a 204, and callers that ignored the resolved value).
func (c *githubClient) do(method, urlPath string, body any, out any) error {
	var reqBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body for %s: %w", urlPath, err)
		}
		reqBody = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, githubAPIBase+urlPath, reqBody)
	if err != nil {
		return fmt.Errorf("building request for %s: %w", urlPath, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("GitHub API %s failed: %w", urlPath, err)
	}
	defer res.Body.Close()

	respBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading response body for %s: %w", urlPath, err)
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("GitHub API %s failed: %d %s", urlPath, res.StatusCode, string(respBytes))
	}

	if res.StatusCode == http.StatusNoContent || out == nil || len(respBytes) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBytes, out); err != nil {
		return fmt.Errorf("decoding response for %s: %w", urlPath, err)
	}
	return nil
}

func (c *githubClient) get(urlPath string, out any) error {
	return c.do(http.MethodGet, urlPath, nil, out)
}

func (c *githubClient) post(urlPath string, body any, out any) error {
	return c.do(http.MethodPost, urlPath, body, out)
}
