package platform

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
)

func IsWindows() bool {
	return runtime.GOOS == "windows"
}

func IsMacOS() bool {
	return runtime.GOOS == "darwin"
}

func IsLinux() bool {
	return runtime.GOOS == "linux"
}

func ExpandPath(path string) string {
	if path != "" && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			if len(path) == 1 {
				return home
			}

			if path[1] == '/' || path[1] == '\\' {
				return filepath.Join(home, path[2:])
			}
		} else {
			log.Printf("Warning: failed to expand ~ in path %q: %v", path, err)
		}
	}

	path = os.ExpandEnv(path)

	if absPath, err := filepath.Abs(path); err == nil {
		return absPath
	}

	return path
}

func UserHomeDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}

	return ""
}
