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

	if version, ok := readVersionFile(); ok {
		return version
	}

	return "dev"
}

func readVersionFile() (string, bool) {
	for _, candidate := range versionFileCandidates() {
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
	candidates := []string{"VERSION"}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return candidates
	}

	dir := filepath.Dir(filename)

	return append(candidates,
		filepath.Join(dir, "..", "..", "VERSION"),
		filepath.Join(dir, "..", "..", "..", "VERSION"),
	)
}
