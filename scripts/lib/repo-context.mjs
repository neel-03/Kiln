/**
 * Shared helpers for building bounded, full-content repo context. Both scripts
 * run after a full `actions/checkout`, so — unlike a diff-only review —
 * they can hand the model the complete text of the files that matter
 * instead of just paths or diff hunks.
 */
import fs from "node:fs";
import path from "node:path";

const DEFAULT_EXCLUDE_DIRS = new Set([
  ".git",
  "node_modules",
  "vendor",
  "dist",
  ".kiln",
]);

/** Walk `root` and return an indented tree string, skipping noisy dirs. */
export function buildRepoTree(root, { excludeDirs = DEFAULT_EXCLUDE_DIRS, maxEntries = 1500 } = {}) {
  const lines = [];
  let count = 0;

  function walk(dir, depth) {
    if (count >= maxEntries) return;
    let entries;
    try {
      entries = fs.readdirSync(dir, { withFileTypes: true });
    } catch {
      return;
    }
    entries.sort((a, b) => a.name.localeCompare(b.name));
    for (const entry of entries) {
      if (count >= maxEntries) return;
      if (entry.name === ".git") continue;
      if (entry.isDirectory() && excludeDirs.has(entry.name)) continue;
      lines.push(`${"  ".repeat(depth)}${entry.name}${entry.isDirectory() ? "/" : ""}`);
      count++;
      if (entry.isDirectory()) walk(path.join(dir, entry.name), depth + 1);
    }
  }

  walk(root, 0);
  if (count >= maxEntries) lines.push(`... (truncated at ${maxEntries} entries)`);
  return lines.join("\n");
}

/** Read a file's content, capped at maxBytes, returning null if missing/unreadable. */
export function readFileSafe(filePath, maxBytes = 20_000) {
  try {
    const buf = fs.readFileSync(filePath);
    if (buf.length <= maxBytes) return buf.toString("utf8");
    return `${buf.subarray(0, maxBytes).toString("utf8")}\n... (truncated, ${buf.length} bytes total)`;
  } catch {
    return null;
  }
}

/**
 * Assemble labeled full-text sections for `relativePaths` under `root`,
 * stopping once `maxTotalBytes` is reached rather than growing unbounded.
 */
export function collectFileContents(root, relativePaths, { maxTotalBytes = 300_000, maxFileBytes = 40_000 } = {}) {
  const sections = [];
  let used = 0;
  for (const rel of relativePaths) {
    if (used >= maxTotalBytes) {
      sections.push(`(skipped remaining files — repo context byte limit of ${maxTotalBytes} reached)`);
      break;
    }
    const content = readFileSafe(path.join(root, rel), maxFileBytes);
    if (content === null) continue;
    const section = `--- ${rel} ---\n${content}`;
    used += section.length;
    sections.push(section);
  }
  return sections.join("\n\n");
}
