//go:build windows

package instance

import (
	"log"
	"os"
	"path/filepath"

	"github.com/debrief-dev/debrief/infra/config"
)

func socketPath() string {
	dir, err := config.AppDirectory()
	if err != nil {
		log.Printf("instance: cannot determine app directory: %v", err)
		return filepath.Join(os.TempDir(), "debrief.sock")
	}

	return filepath.Join(dir, "debrief.sock")
}

func socketNetwork() string {
	return "unix"
}
