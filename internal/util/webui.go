package util

import (
	"os"
	"path/filepath"
	"strings"
)

// GetWebUIVersion reads the WebUI version tag from the 'revision' file on disk.
// Returns "r3541" fallback if the file cannot be read.
func GetWebUIVersion(rootDir string) string {
	paths := []string{
		filepath.Join(rootDir, "webUI", "revision"),
		filepath.Join(".", "data", "webUI", "revision"),
		"/opt/tsuruyomi/data/webUI/revision",
		"/opt/gosho/data/webUI/revision",
		"/root/.local/share/Tachidesk/webUI/revision",
	}

	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil {
			rev := strings.TrimSpace(string(data))
			if rev != "" {
				return rev
			}
		}
	}

	return "r3541"
}
