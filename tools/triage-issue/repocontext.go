package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var defaultExcludeDirs = map[string]bool{
	".git":   true,
	"vendor": true,
	".kiln":  true,
}

const defaultMaxTreeEntries = 1500

// buildRepoTree walks root and returns an indented tree string, skipping
// noisy directories
func buildRepoTree(root string) string {
	var lines []string
	count := 0

	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		if count >= defaultMaxTreeEntries {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name() < entries[j].Name()
		})
		for _, entry := range entries {
			if count >= defaultMaxTreeEntries {
				return
			}
			if entry.Name() == ".git" {
				continue
			}
			if entry.IsDir() && defaultExcludeDirs[entry.Name()] {
				continue
			}
			suffix := ""
			if entry.IsDir() {
				suffix = "/"
			}
			lines = append(lines, strings.Repeat("  ", depth)+entry.Name()+suffix)
			count++
			if entry.IsDir() {
				walk(filepath.Join(dir, entry.Name()), depth+1)
			}
		}
	}

	walk(root, 0)
	if count >= defaultMaxTreeEntries {
		lines = append(lines, fmt.Sprintf("... (truncated at %d entries)", defaultMaxTreeEntries))
	}
	return strings.Join(lines, "\n")
}

// readFileSafe reads a file's content capped at maxBytes, returning
// (content, true) on success or ("", false) if the file is missing or
// unreadable -- a port of readFileSafe(), using an (string, bool) result
// instead of JS's null-sentinel since Go has no natural "nullable string".
func readFileSafe(filePath string, maxBytes int) (string, bool) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", false
	}
	if len(data) <= maxBytes {
		return string(data), true
	}
	return fmt.Sprintf("%s\n... (truncated, %d bytes total)", string(data[:maxBytes]), len(data)), true
}

const defaultMaxFileBytes = 40_000

// collectFileContents assembles labeled full-text sections for
// relativePaths under root, stopping once maxTotalBytes is reached --
// a port of collectFileContents().
func collectFileContents(root string, relativePaths []string, maxTotalBytes int) string {
	var sections []string
	used := 0
	for _, rel := range relativePaths {
		if used >= maxTotalBytes {
			sections = append(sections, fmt.Sprintf("(skipped remaining files — repo context byte limit of %d reached)", maxTotalBytes))
			break
		}
		content, ok := readFileSafe(filepath.Join(root, rel), defaultMaxFileBytes)
		if !ok {
			continue
		}
		section := fmt.Sprintf("--- %s ---\n%s", rel, content)
		used += len(section)
		sections = append(sections, section)
	}
	return strings.Join(sections, "\n\n")
}
