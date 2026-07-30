/**
 * Shared GitHub REST API client for the automation scripts.
 */

export function requireEnv(name) {
  const v = process.env[name];
  if (!v) {
    console.error(`Missing required env var: ${name}`);
    process.exit(1);
  }
  return v;
}

const GITHUB_API = "https://api.github.com";

/** Returns a `githubRequest(path, options)` function bound to one token. */
export function makeGitHubClient(token) {
  return async function githubRequest(urlPath, options = {}) {
    const res = await fetch(`${GITHUB_API}${urlPath}`, {
      ...options,
      headers: {
        Authorization: `Bearer ${token}`,
        Accept: "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
        ...(options.headers || {}),
      },
    });
    if (!res.ok) {
      const body = await res.text();
      throw new Error(`GitHub API ${urlPath} failed: ${res.status} ${body}`);
    }
    return res.status === 204 ? null : res.json();
  };
}
