//go:build linux

package hotkey

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

// D-Bus portal constants
const (
	portalDest         = "org.freedesktop.portal.Desktop"
	portalPath         = "/org/freedesktop/portal/desktop"
	portalShortcutIf   = "org.freedesktop.portal.GlobalShortcuts"
	portalRequestIf    = "org.freedesktop.portal.Request"
	portalSessionIf    = "org.freedesktop.portal.Session"
	shortcutID         = "debrief-toggle"
	shortcutDesc       = "Toggle Debrief window"
	responseTimeout    = 5 * time.Second
	signalBufSize      = 4
	minResponseBodyLen = 2
)

// portalBackend uses the D-Bus GlobalShortcuts portal for Wayland hotkeys.
type portalBackend struct {
	conn             *dbus.Conn
	sessionPath      dbus.ObjectPath
	preferredTrigger string
	keyChan          chan struct{}
	mu               sync.Mutex
	closed           bool
}

// newPortalBackend creates a portal backend and verifies the portal is available.
func newPortalBackend(modStrs []string, keyStr string) (*portalBackend, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to session bus: %w", err)
	}

	// Verify GlobalShortcuts interface exists via introspection.
	// A simple Ping only checks that the portal daemon is running;
	// GNOME's xdg-desktop-portal-gnome responds to Ping but does NOT
	// implement GlobalShortcuts, causing CreateSession to fail later.
	obj := conn.Object(portalDest, dbus.ObjectPath(portalPath))

	node, err := introspect.Call(obj)
	if err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("Hotkey portal: error closing D-Bus connection: %v", closeErr)
		}

		return nil, fmt.Errorf("failed to introspect portal: %w", err)
	}

	found := false

	for _, iface := range node.Interfaces {
		if iface.Name == portalShortcutIf {
			found = true
			break
		}
	}

	if !found {
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("Hotkey portal: error closing D-Bus connection: %v", closeErr)
		}

		return nil, errors.New("GlobalShortcuts interface not available on this portal backend")
	}

	return &portalBackend{
		conn:             conn,
		preferredTrigger: buildTriggerString(modStrs, keyStr),
		keyChan:          make(chan struct{}, 1),
	}, nil
}

// Register creates a portal session, binds the shortcut, and starts listening.
func (p *portalBackend) Register() error {
	senderName := p.conn.Names()[0]
	// D-Bus sender in path: replace ":" with "" and "." with "_"
	senderToken := strings.ReplaceAll(strings.TrimPrefix(senderName, ":"), ".", "_")

	sessionToken := "debrief_session"
	handleToken := "debrief_req"

	// Subscribe to Request.Response signals before making calls
	if err := p.conn.AddMatchSignal(
		dbus.WithMatchInterface(portalRequestIf),
		dbus.WithMatchMember("Response"),
	); err != nil {
		return fmt.Errorf("failed to add signal match: %w", err)
	}

	sigChan := make(chan *dbus.Signal, signalBufSize)
	p.conn.Signal(sigChan)

	// Step 1: CreateSession
	obj := p.conn.Object(portalDest, dbus.ObjectPath(portalPath))

	createOpts := map[string]dbus.Variant{
		"handle_token":         dbus.MakeVariant(handleToken),
		"session_handle_token": dbus.MakeVariant(sessionToken),
	}

	call := obj.Call(portalShortcutIf+".CreateSession", 0, createOpts)
	if call.Err != nil {
		return fmt.Errorf("CreateSession call failed: %w", call.Err)
	}

	expectedReqPath := dbus.ObjectPath(
		fmt.Sprintf("/org/freedesktop/portal/desktop/request/%s/%s", senderToken, handleToken),
	)

	responseCode, results, err := waitResponse(sigChan, expectedReqPath)
	if err != nil {
		return fmt.Errorf("CreateSession response failed: %w", err)
	}

	if responseCode != 0 {
		return fmt.Errorf("CreateSession denied (response code: %d)", responseCode)
	}

	// Extract session handle from results
	sessionHandleVariant, ok := results["session_handle"]
	if !ok {
		return errors.New("CreateSession response missing session_handle")
	}

	sessionHandle, ok := sessionHandleVariant.Value().(string)
	if !ok {
		return errors.New("session_handle is not a string")
	}

	p.sessionPath = dbus.ObjectPath(sessionHandle)

	log.Printf("Hotkey portal: Session created: %s", p.sessionPath)

	// Step 2: BindShortcuts
	bindHandleToken := "debrief_bind"

	shortcutOpts := map[string]dbus.Variant{
		"description":       dbus.MakeVariant(shortcutDesc),
		"preferred_trigger": dbus.MakeVariant(p.preferredTrigger),
	}

	type shortcutEntry struct {
		ID      string
		Options map[string]dbus.Variant
	}

	shortcuts := []shortcutEntry{
		{ID: shortcutID, Options: shortcutOpts},
	}

	bindOpts := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(bindHandleToken),
	}

	call = obj.Call(portalShortcutIf+".BindShortcuts", 0,
		p.sessionPath, shortcuts, "", bindOpts)
	if call.Err != nil {
		return fmt.Errorf("BindShortcuts call failed: %w", call.Err)
	}

	expectedBindPath := dbus.ObjectPath(
		fmt.Sprintf("/org/freedesktop/portal/desktop/request/%s/%s", senderToken, bindHandleToken),
	)

	responseCode, _, err = waitResponse(sigChan, expectedBindPath)
	if err != nil {
		return fmt.Errorf("BindShortcuts response failed: %w", err)
	}

	if responseCode != 0 {
		return fmt.Errorf("BindShortcuts denied (response code: %d)", responseCode)
	}

	log.Printf("Hotkey portal: Shortcut bound with preferred trigger: %s", p.preferredTrigger)

	// Stop receiving Request.Response signals, switch to Activated
	p.conn.RemoveSignal(sigChan)

	if err := p.conn.RemoveMatchSignal(
		dbus.WithMatchInterface(portalRequestIf),
		dbus.WithMatchMember("Response"),
	); err != nil {
		log.Printf("Hotkey portal: warning: failed to remove Response match: %v", err)
	}

	// Step 3: Listen for Activated signals
	go p.listenActivated()

	return nil
}

// listenActivated listens for GlobalShortcuts.Activated signals on D-Bus.
func (p *portalBackend) listenActivated() {
	if err := p.conn.AddMatchSignal(
		dbus.WithMatchInterface(portalShortcutIf),
		dbus.WithMatchMember("Activated"),
	); err != nil {
		log.Printf("Hotkey portal: failed to subscribe to Activated signal: %v", err)
		return
	}

	sigChan := make(chan *dbus.Signal, signalBufSize)
	p.conn.Signal(sigChan)

	log.Println("Hotkey portal: Listening for Activated signals")

	for sig := range sigChan {
		if sig.Name != portalShortcutIf+".Activated" {
			continue
		}

		// Activated signal body: (session_handle, shortcut_id, timestamp, options)
		if len(sig.Body) < minResponseBodyLen {
			continue
		}

		id, ok := sig.Body[1].(string)
		if !ok || id != shortcutID {
			continue
		}

		log.Println("Hotkey portal: Shortcut activated")

		select {
		case p.keyChan <- struct{}{}:
		default:
			// Channel full, skip duplicate
		}
	}
}

// Unregister closes the portal session and D-Bus connection.
func (p *portalBackend) Unregister() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true

	// Close the portal session
	if p.sessionPath != "" {
		sessionObj := p.conn.Object(portalDest, p.sessionPath)

		if err := sessionObj.Call(portalSessionIf+".Close", 0).Err; err != nil {
			log.Printf("Hotkey portal: warning: failed to close session: %v", err)
		}
	}

	if err := p.conn.Close(); err != nil {
		return fmt.Errorf("failed to close D-Bus connection: %w", err)
	}

	log.Println("Hotkey portal: Session closed")

	return nil
}

// Keydown returns the channel that receives hotkey activation events.
func (p *portalBackend) Keydown() <-chan struct{} {
	return p.keyChan
}

// waitResponse waits for a portal Response signal on the expected request path.
func waitResponse(sigChan <-chan *dbus.Signal, expectedPath dbus.ObjectPath) (uint32, map[string]dbus.Variant, error) {
	timer := time.NewTimer(responseTimeout)
	defer timer.Stop()

	for {
		select {
		case sig, ok := <-sigChan:
			if !ok {
				return 0, nil, errors.New("signal channel closed")
			}

			if sig.Name != portalRequestIf+".Response" {
				continue
			}

			if sig.Path != expectedPath {
				continue
			}

			if len(sig.Body) < minResponseBodyLen {
				return 0, nil, errors.New("response signal has insufficient body fields")
			}

			code, ok := sig.Body[0].(uint32)
			if !ok {
				return 0, nil, errors.New("response code is not uint32")
			}

			results, _ := sig.Body[1].(map[string]dbus.Variant)

			return code, results, nil

		case <-timer.C:
			return 0, nil, errors.New("timed out waiting for portal response")
		}
	}
}

// buildTriggerString converts modifier strings and a key to the portal trigger format.
// Example output: "CTRL+SHIFT+h"
func buildTriggerString(modStrs []string, keyStr string) string {
	parts := make([]string, 0, len(modStrs)+1)

	for _, mod := range modStrs {
		switch mod {
		case Ctrl:
			parts = append(parts, "CTRL")
		case Shift:
			parts = append(parts, "SHIFT")
		case Alt:
			parts = append(parts, "ALT")
		case Win, Cmd:
			parts = append(parts, "SUPER")
		default:
			parts = append(parts, strings.ToUpper(mod))
		}
	}

	parts = append(parts, strings.ToLower(keyStr))

	return strings.Join(parts, "+")
}
