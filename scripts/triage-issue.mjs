#!/usr/bin/env node
/**
 * Triage a newly-opened GitHub issue using Gemini's API:
 * Required env vars:
 *   GITHUB_TOKEN     - provided automatically inside GitHub Actions
 *   GEMINI_API_KEY   - api-key stored as a repo secret
 *   GITHUB_REPOSITORY - "owner/repo", provided automatically
 *   ISSUE_NUMBER     - the issue that triggered this run
 *
 * Requires Node 20+ (built-in fetch, no dependencies) and a full
 * `actions/checkout` before this script runs (see issue-triage.yml).
 */
import { requireEnv, makeGitHubClient } from "./lib/github.mjs";
import { buildRepoTree, readFileSafe, collectFileContents } from "./lib/repo-context.mjs";

const GITHUB_TOKEN = requireEnv("GITHUB_TOKEN");
const GEMINI_API_KEY = requireEnv("GEMINI_API_KEY");
const [OWNER, REPO] = requireEnv("GITHUB_REPOSITORY").split("/");
const ISSUE_NUMBER = requireEnv("ISSUE_NUMBER");

const githubRequest = makeGitHubClient(GITHUB_TOKEN);
const GEMINI_MODEL = "gemini-flash-latest";

const RULES_FILE = ".gemini/skills/kiln/SKILL.md";
const MAX_CONTEXT_FILES = 6;

const ALLOWED_LABELS = [
  "bug",
  "feature request",
  "documentation",
  "needs more info",
  "good first issue",
  "needs design review",
];

async function fetchIssue() {
  return githubRequest(`/repos/${OWNER}/${REPO}/issues/${ISSUE_NUMBER}`);
}

/**
 * repo-context gathering: pull 2-3 significant words out of the issue
 * title/body and run them through GitHub's code search, scoped to this repo.
 * It's enough signal to point at the right handful of files, whose full content
 * gets read next.
 */
async function findRelatedFiles(issue) {
  const text = `${issue.title} ${issue.body ?? ""}`;
  const words = text
    .toLowerCase()
    .replace(/[^a-z0-9_./\- ]/g, " ")
    .split(/\s+/)
    .filter((w) => w.length > 4)
    .filter((w) => !STOPWORDS.has(w));

  const uniqueWords = [...new Set(words)].slice(0, 3);
  if (uniqueWords.length === 0) return [];

  const query = `${uniqueWords.join(" ")} repo:${OWNER}/${REPO}`;
  try {
    const results = await githubRequest(
      `/search/code?q=${encodeURIComponent(query)}&per_page=${MAX_CONTEXT_FILES}`
    );
    return (results.items || []).map((item) => item.path);
  } catch (err) {
    // Code search can 422 on very short/common queries — non-fatal, just
    // means the model gets less context, not that triage fails entirely.
    console.warn("Code search skipped:", err.message);
    return [];
  }
}

const STOPWORDS = new Set([
  "about", "after", "again", "before", "could", "should", "would",
  "there", "which", "these", "those", "where", "while", "using",
]);

async function askGemini(issue, relatedFiles, repoTree, rules, relatedFileContents) {
  const prompt = `You are triaging a GitHub issue for "Kiln", a Go-based,
language-agnostic project-orchestration tool (config layering, patchable
templates, a plugin system with three trust tiers, and pluggable deployment
targets like Docker Compose / Kubernetes / systemd).

Issue title: ${issue.title}

Issue body:
${issue.body ?? "(no body provided)"}

Repo directory structure (for orientation — most of this may not be
relevant to this specific issue):
${repoTree}

Kiln's hard design rules and conventions (${RULES_FILE}). Treat anything
that looks like it would violate a rule in the "Hard rules" section below
as grounds for the "needs-design-review" label, and say why in the comment:
${rules ?? "(rules file not found)"}

Files that matched keywords from this issue, with their full current
content (may or may not be actually relevant — use your judgement):
${relatedFiles.length ? relatedFileContents : "(no related files found)"}

Respond with ONLY a JSON object, no markdown fences, no extra text, matching
exactly this shape:
{
  "labels": ["..."],           // choose only from: ${ALLOWED_LABELS.join(", ")}
  "comment": "..."             // a short, helpful triage comment, max ~120 words,
                                 // written to the issue author, plain markdown
}`;

  const res = await fetch(
    `https://generativelanguage.googleapis.com/v1beta/models/${GEMINI_MODEL}:generateContent`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "x-goog-api-key": GEMINI_API_KEY,
      },
      body: JSON.stringify({
        contents: [{ parts: [{ text: prompt }] }],
        generationConfig: {
          temperature: 0.2,
          responseMimeType: "application/json",
        },
      }),
    }
  );

  if (!res.ok) {
    throw new Error(`Gemini API failed: ${res.status} ${await res.text()}`);
  }

  const data = await res.json();
  const text = data.candidates?.[0]?.content?.parts?.[0]?.text;
  if (!text) throw new Error(`Unexpected Gemini response shape: ${JSON.stringify(data)}`);

  const parsed = JSON.parse(text);
  const safeLabels = (parsed.labels || []).filter((l) => ALLOWED_LABELS.includes(l));
  return { labels: safeLabels, comment: parsed.comment || "" };
}

async function applyTriage({ labels, comment }) {
  if (labels.length > 0) {
    await githubRequest(`/repos/${OWNER}/${REPO}/issues/${ISSUE_NUMBER}/labels`, {
      method: "POST",
      body: JSON.stringify({ labels }),
    });
  }
  if (comment.trim()) {
    await githubRequest(`/repos/${OWNER}/${REPO}/issues/${ISSUE_NUMBER}/comments`, {
      method: "POST",
      body: JSON.stringify({
        body: `${comment.trim()}\n\n---\n*Automated triage — a human maintainer will follow up.*`,
      }),
    });
  }
}

async function main() {
  const issue = await fetchIssue();
  const relatedFiles = await findRelatedFiles(issue);
  const repoTree = buildRepoTree(process.cwd());
  const rules = readFileSafe(RULES_FILE);
  const relatedFileContents = collectFileContents(process.cwd(), relatedFiles, { maxTotalBytes: 150_000 });
  const triage = await askGemini(issue, relatedFiles, repoTree, rules, relatedFileContents);
  console.log("Triage result:", JSON.stringify(triage, null, 2));
  await applyTriage(triage);
  console.log(`Applied triage to #${ISSUE_NUMBER}`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
