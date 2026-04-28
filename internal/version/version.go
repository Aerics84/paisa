package version

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var current string

func String() string {
	if current != "" {
		return current
	}

	resolved := resolveVersion("", versionFileCandidates())
	if resolved != "dev" {
		current = resolved
	}

	return resolved
}

func resolveVersion(injected string, candidates []string) string {
	if injected != "" {
		return injected
	}

	if version, ok := lookupVersion(candidates); ok {
		return version
	}

	return "dev"
}

func readVersionFile() (string, bool) {
	return lookupVersion(versionFileCandidates())
}

func lookupVersion(candidates []string) (string, bool) {
	for _, candidate := range candidates {
		content, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}

		version := strings.TrimSpace(string(content))
		if version != "" {
			return version, true
		}
	}

	return "", false
}

func versionFileCandidates() []string {
	seen := map[string]struct{}{}
	candidates := make([]string, 0, 8)

	addCandidate := func(path string) {
		if path == "" {
			return
		}

		cleaned := filepath.Clean(path)
		if _, ok := seen[cleaned]; ok {
			return
		}

		seen[cleaned] = struct{}{}
		candidates = append(candidates, cleaned)
	}

	addCandidate("VERSION")

	_, filename, _, ok := runtime.Caller(0)
	if ok {
		dir := filepath.Dir(filename)
		addCandidate(filepath.Join(dir, "..", "..", "VERSION"))
		addCandidate(filepath.Join(dir, "..", "..", "..", "VERSION"))
	}

	if executable, err := os.Executable(); err == nil {
		dir := filepath.Dir(executable)
		addCandidate(filepath.Join(dir, "VERSION"))
		addCandidate(filepath.Join(dir, "..", "VERSION"))
		addCandidate(filepath.Join(dir, "..", "..", "VERSION"))
	}

	return candidates
}
