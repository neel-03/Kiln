// Command triage-issue triages a newly-opened GitHub issue using Gemini's
// API, with full repo context (directory tree, the project's rules file,
// and the full content of a handful of keyword-matched files).
//
// Required env vars:
//
//	GITHUB_TOKEN       - provided automatically inside GitHub Actions
//	GEMINI_API_KEY     - API key stored as a repo secret
//	GITHUB_REPOSITORY  - "owner/repo", provided automatically
//	ISSUE_NUMBER       - the issue that triggered this run
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
)

var (
	geminiModel        = "gemini-flash-latest"
	geminiEndpointTmpl = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent"
)

const (
	rulesFile                 = ".gemini/skills/kiln/SKILL.md"
	maxContextFiles           = 6
	rulesFileBytes            = 20_000 // matches readFileSafe's own default; kept explicit here for clarity
	relatedFilesMaxTotalBytes = 150_000
)

var allowedLabels = []string{
	"bug",
	"feature request",
	"documentation",
	"needs more info",
	"good first issue",
	"needs design review",
}

var stopwords = map[string]bool{
	"about": true, "after": true, "again": true, "before": true,
	"could": true, "should": true, "would": true, "there": true,
	"which": true, "these": true, "those": true, "where": true,
	"while": true, "using": true,
}

// nonWordChar matches the original's /[^a-z0-9_./\- ]/g character class,
// with the hyphen moved to the end of the bracket expression so it doesn't
// need escaping in Go's RE2 syntax -- functionally identical.
var nonWordChar = regexp.MustCompile(`[^a-z0-9_./ -]`)

type issue struct {
	Title string  `json:"title"`
	Body  *string `json:"body"`
}

func (i issue) bodyOr(fallback string) string {
	if i.Body == nil {
		return fallback
	}
	return *i.Body
}

type codeSearchResult struct {
	Items []struct {
		Path string `json:"path"`
	} `json:"items"`
}

type triageResult struct {
	Labels  []string `json:"labels"`
	Comment string   `json:"comment"`
}

func fetchIssue(client *githubClient, owner, repo, issueNumber string) (issue, error) {
	var i issue
	err := client.get(fmt.Sprintf("/repos/%s/%s/issues/%s", owner, repo, issueNumber), &i)
	return i, err
}

// findRelatedFiles pulls 2-3 significant words out of the issue title/body
// and runs them through GitHub's code search, scoped to this repo.
func findRelatedFiles(client *githubClient, owner, repo string, iss issue) []string {
	text := strings.ToLower(iss.Title + " " + iss.bodyOr(""))
	text = nonWordChar.ReplaceAllString(text, " ")

	var unique []string
	seen := map[string]bool{}
	for _, w := range strings.Fields(text) {
		if len(w) <= 4 || stopwords[w] || seen[w] {
			continue
		}
		seen[w] = true
		unique = append(unique, w)
		if len(unique) == 3 {
			break
		}
	}
	if len(unique) == 0 {
		return nil
	}

	query := fmt.Sprintf("%s repo:%s/%s", strings.Join(unique, " "), owner, repo)
	var results codeSearchResult
	err := client.get(fmt.Sprintf("/search/code?q=%s&per_page=%d", url.QueryEscape(query), maxContextFiles), &results)
	if err != nil {
		// Code search can 422 on very short/common queries -- non-fatal,
		// just means the model gets less context, not that triage fails.
		fmt.Fprintf(os.Stderr, "Code search skipped: %v\n", err)
		return nil
	}

	paths := make([]string, 0, len(results.Items))
	for _, item := range results.Items {
		paths = append(paths, item.Path)
	}
	return paths
}

type geminiRequest struct {
	Contents         []geminiContent `json:"contents"`
	GenerationConfig geminiGenConfig `json:"generationConfig"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenConfig struct {
	Temperature      float64 `json:"temperature"`
	ResponseMimeType string  `json:"responseMimeType"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func askGemini(apiKey string, iss issue, relatedFiles []string, repoTree string, rules string, hasRules bool, relatedFileContents string) (triageResult, error) {
	rulesSection := rules
	if !hasRules {
		rulesSection = "(rules file not found)"
	}

	filesSection := "(no related files found)"
	if len(relatedFiles) > 0 {
		filesSection = relatedFileContents
	}

	prompt := fmt.Sprintf(`You are triaging a GitHub issue for "Kiln", a Go-based,
language-agnostic project-orchestration tool (config layering, patchable
templates, a plugin system with three trust tiers, and pluggable deployment
targets like Docker Compose / Kubernetes / systemd).

Issue title: %s

Issue body:
%s

Repo directory structure (for orientation — most of this may not be
relevant to this specific issue):
%s

Kiln's hard design rules and conventions (%s). Treat anything
that looks like it would violate a rule in the "Hard rules" section below
as grounds for the "needs-design-review" label, and say why in the comment:
%s

Files that matched keywords from this issue, with their full current
content (may or may not be actually relevant — use your judgement):
%s

Respond with ONLY a JSON object, no markdown fences, no extra text, matching
exactly this shape:
{
  "labels": ["..."],           // choose only from: %s
  "comment": "..."             // a short, helpful triage comment, max ~120 words,
                                 // written to the issue author, plain markdown
}`,
		iss.Title,
		iss.bodyOr("(no body provided)"),
		repoTree,
		rulesFile,
		rulesSection,
		filesSection,
		strings.Join(allowedLabels, ", "),
	)

	reqBody := geminiRequest{
		Contents: []geminiContent{{Parts: []geminiPart{{Text: prompt}}}},
		GenerationConfig: geminiGenConfig{
			Temperature:      0.2,
			ResponseMimeType: "application/json",
		},
	}
	encoded, err := json.Marshal(reqBody)
	if err != nil {
		return triageResult{}, fmt.Errorf("encoding gemini request: %w", err)
	}

	endpoint := fmt.Sprintf(geminiEndpointTmpl, geminiModel)
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(string(encoded)))
	if err != nil {
		return triageResult{}, fmt.Errorf("building gemini request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return triageResult{}, fmt.Errorf("gemini API request failed: %w", err)
	}
	defer res.Body.Close()

	respBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return triageResult{}, fmt.Errorf("reading gemini response: %w", err)
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return triageResult{}, fmt.Errorf("gemini API failed: %d %s", res.StatusCode, string(respBytes))
	}

	var parsed geminiResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return triageResult{}, fmt.Errorf("decoding gemini response: %w", err)
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return triageResult{}, fmt.Errorf("unexpected gemini response shape: %s", string(respBytes))
	}
	rawText := parsed.Candidates[0].Content.Parts[0].Text

	var result triageResult
	if err := json.Unmarshal([]byte(rawText), &result); err != nil {
		return triageResult{}, fmt.Errorf("gemini returned invalid JSON: %w (raw: %s)", err, rawText)
	}

	allowed := make(map[string]bool, len(allowedLabels))
	for _, l := range allowedLabels {
		allowed[l] = true
	}
	safeLabels := make([]string, 0, len(result.Labels))
	for _, l := range result.Labels {
		if allowed[l] {
			safeLabels = append(safeLabels, l)
		}
	}
	result.Labels = safeLabels
	return result, nil
}

func applyTriage(client *githubClient, owner, repo, issueNumber string, result triageResult) error {
	if len(result.Labels) > 0 {
		body := map[string][]string{"labels": result.Labels}
		if err := client.post(fmt.Sprintf("/repos/%s/%s/issues/%s/labels", owner, repo, issueNumber), body, nil); err != nil {
			return err
		}
	}
	comment := strings.TrimSpace(result.Comment)
	if comment != "" {
		body := map[string]string{
			"body": comment + "\n\n---\n*Automated triage — a human maintainer will follow up.*",
		}
		if err := client.post(fmt.Sprintf("/repos/%s/%s/issues/%s/comments", owner, repo, issueNumber), body, nil); err != nil {
			return err
		}
	}
	return nil
}

func run() error {
	githubToken := requireEnv("GITHUB_TOKEN")
	geminiAPIKey := requireEnv("GEMINI_API_KEY")
	repoFull := requireEnv("GITHUB_REPOSITORY")
	issueNumber := requireEnv("ISSUE_NUMBER")

	parts := strings.SplitN(repoFull, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("GITHUB_REPOSITORY must be in owner/repo form, got %q", repoFull)
	}
	owner, repo := parts[0], parts[1]

	client := newGitHubClient(githubToken)

	iss, err := fetchIssue(client, owner, repo, issueNumber)
	if err != nil {
		return fmt.Errorf("fetching issue: %w", err)
	}

	relatedFiles := findRelatedFiles(client, owner, repo, iss)

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	repoTree := buildRepoTree(cwd)
	rules, hasRules := readFileSafe(rulesFile, rulesFileBytes)
	relatedFileContents := collectFileContents(cwd, relatedFiles, relatedFilesMaxTotalBytes)

	result, err := askGemini(geminiAPIKey, iss, relatedFiles, repoTree, rules, hasRules, relatedFileContents)
	if err != nil {
		return fmt.Errorf("asking gemini: %w", err)
	}

	pretty, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling result for logging: %w", err)
	}
	fmt.Printf("Triage result: %s\n", pretty)

	if err := applyTriage(client, owner, repo, issueNumber, result); err != nil {
		return fmt.Errorf("applying triage: %w", err)
	}
	fmt.Printf("Applied triage to #%s\n", issueNumber)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
