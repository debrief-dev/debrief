//go:build !windows

package instance

import (
	"log"
	"path/filepath"

	"github.com/debrief-dev/debrief/infra/config"
)

func socketPath() string {
	dir, err := config.AppDirectory()
	if err != nil {
		log.Printf("instance: cannot determine app directory: %v, falling back to /tmp", err)
		return "/tmp/debrief.sock"
	}

	return filepath.Join(dir, "debrief.sock")
}

func socketNetwork() string {
	return "unix"
}
