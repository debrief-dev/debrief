package instance

import (
	"bufio"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/debrief-dev/debrief/infra/config"
)

// connTimeout bounds how long an accepted or dialed connection may be idle.
const connTimeout = 2 * time.Second

// TryAcquire attempts to become the single running instance.
// If another instance is already listening, it sends a "show" command and
// returns (false, nil) — the caller should exit.
// If this is the first instance, it starts a listener and returns
// (true, cleanup) where cleanup closes the listener and removes the socket.
func TryAcquire(windowSignalChan chan<- string) (acquired bool, cleanup func()) {
	path := socketPath()
	network := socketNetwork()

	// Try to connect to an existing instance.
	if sendShow(network, path) {
		return false, nil
	}

	// No listener found — remove stale socket file and start listening.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		log.Printf("instance: failed to remove stale socket %s: %v", path, err)
	}

	ln, err := net.Listen(network, path) //nolint:noctx // no cancellation needed for a long-lived listener
	if err != nil {
		// Race: another instance grabbed the socket between our Dial and Listen.
		// Retry connecting once.
		if sendShow(network, path) {
			return false, nil
		}

		log.Printf("instance: failed to listen on %s: %v (continuing without single-instance guard)", path, err)

		return true, nil
	}

	// Restrict socket permissions to owner only.
	if err := os.Chmod(path, config.FilePermissions); err != nil {
		log.Printf("instance: failed to chmod socket %s: %v", path, err)
	}

	go acceptLoop(ln, windowSignalChan)

	return true, func() {
		if err := ln.Close(); err != nil {
			log.Printf("instance: failed to close listener: %v", err)
		}

		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			log.Printf("instance: failed to remove socket %s: %v", path, err)
		}
	}
}

// acceptLoop accepts connections and forwards "show" commands to the signal channel.
func acceptLoop(ln net.Listener, windowSignalChan chan<- string) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			// Listener closed (normal shutdown).
			return
		}

		go handleConn(conn, windowSignalChan)
	}
}

// handleConn reads commands from a connection and forwards them.
func handleConn(conn net.Conn, windowSignalChan chan<- string) {
	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("instance: failed to close connection: %v", err)
		}
	}()

	if err := conn.SetDeadline(time.Now().Add(connTimeout)); err != nil {
		log.Printf("instance: failed to set connection deadline: %v", err)
		return
	}

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		cmd := strings.TrimSpace(scanner.Text())
		if cmd == "show" {
			select {
			case windowSignalChan <- "show":
			default:
				// Channel full — a signal is already pending.
			}
		}
	}
}

// sendShow tries to connect to an existing instance and send a "show" command.
// Returns true if the command was delivered successfully.
func sendShow(network, path string) bool {
	dialer := net.Dialer{Timeout: connTimeout}

	conn, err := dialer.Dial(network, path) //nolint:noctx // one-shot connection, no cancellation needed
	if err != nil {
		return false
	}

	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("instance: failed to close connection: %v", err)
		}
	}()

	if err := conn.SetDeadline(time.Now().Add(connTimeout)); err != nil {
		log.Printf("instance: failed to set write deadline: %v", err)
		return false
	}

	_, err = conn.Write([]byte("show\n"))

	return err == nil
}
