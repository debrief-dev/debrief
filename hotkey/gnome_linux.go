//go:build linux

package hotkey

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
)

// GNOME gsettings + D-Bus service constants
const (
	gnomeDBusName       = "com.github.debrief"
	gnomeDBusPath       = "/com/github/debrief"
	gnomeDBusIface      = "com.github.debrief"
	gnomeGsettingsBase  = "org.gnome.settings-daemon.plugins.media-keys"
	gnomeCustomKBPath   = "/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/debrief/"
	gnomeShortcutName   = "Toggle Debrief"
	gsettingsEmptyArray = "@as []"
)

// gnomeToggleHandler receives D-Bus Toggle() calls from GNOME's shortcut system.
type gnomeToggleHandler struct {
	keyChan chan<- struct{}
}

// Toggle is called via D-Bus when the GNOME keyboard shortcut fires.
func (h *gnomeToggleHandler) Toggle() *dbus.Error {
	select {
	case h.keyChan <- struct{}{}:
	default:
	}

	return nil
}

// gnomeBackend registers a D-Bus service and a GNOME custom keyboard shortcut
// that sends dbus-send to our service when pressed.
type gnomeBackend struct {
	conn    *dbus.Conn
	modStrs []string
	keyStr  string
	keyChan chan struct{}
	mu      sync.Mutex
	closed  bool
}

// newGnomeBackend creates a GNOME backend if the environment supports it.
func newGnomeBackend(modStrs []string, keyStr string) (*gnomeBackend, error) {
	desktop := os.Getenv("XDG_CURRENT_DESKTOP")
	if !strings.Contains(desktop, "GNOME") {
		return nil, fmt.Errorf("not a GNOME desktop (XDG_CURRENT_DESKTOP=%s)", desktop)
	}

	if _, err := exec.LookPath("gsettings"); err != nil {
		return nil, fmt.Errorf("gsettings not found: %w", err)
	}

	if _, err := exec.LookPath("dbus-send"); err != nil {
		return nil, fmt.Errorf("dbus-send not found: %w", err)
	}

	return &gnomeBackend{
		modStrs: modStrs,
		keyStr:  keyStr,
		keyChan: make(chan struct{}, 1),
	}, nil
}

// Register connects to D-Bus, exports a Toggle method, and configures a
// GNOME custom keyboard shortcut pointing at our D-Bus service.
func (g *gnomeBackend) Register() error {
	// Step 1: Connect to session D-Bus and claim our well-known name
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return fmt.Errorf("failed to connect to session bus: %w", err)
	}

	g.conn = conn

	reply, err := conn.RequestName(gnomeDBusName, dbus.NameFlagDoNotQueue|dbus.NameFlagReplaceExisting)
	if err != nil {
		return fmt.Errorf("failed to request D-Bus name %s: %w", gnomeDBusName, err)
	}

	if reply != dbus.RequestNameReplyPrimaryOwner && reply != dbus.RequestNameReplyAlreadyOwner {
		return fmt.Errorf("failed to acquire D-Bus name %s (reply: %d)", gnomeDBusName, reply)
	}

	// Step 2: Export the Toggle handler
	handler := &gnomeToggleHandler{keyChan: g.keyChan}

	if err := conn.Export(handler, dbus.ObjectPath(gnomeDBusPath), gnomeDBusIface); err != nil {
		return fmt.Errorf("failed to export D-Bus Toggle method: %w", err)
	}

	log.Printf("Hotkey GNOME: D-Bus service registered at %s", gnomeDBusName)

	// Step 3: Build trigger and command strings
	trigger := buildGnomeTrigger(g.modStrs, g.keyStr)

	dbusCmd := fmt.Sprintf(
		"dbus-send --session --type=method_call --dest=%s %s %s.Toggle",
		gnomeDBusName, gnomeDBusPath, gnomeDBusIface,
	)

	// Step 4: Read existing custom-keybindings, append ours if missing
	paths, err := readCustomKeybindings()
	if err != nil {
		return fmt.Errorf("failed to read custom keybindings: %w", err)
	}

	alreadyExists := false

	for _, p := range paths {
		if p == gnomeCustomKBPath {
			alreadyExists = true
			break
		}
	}

	if !alreadyExists {
		paths = append(paths, gnomeCustomKBPath)

		if err := writeCustomKeybindings(paths); err != nil {
			return fmt.Errorf("failed to write custom keybindings: %w", err)
		}
	}

	// Step 5: Configure our shortcut entry
	schemaPath := gnomeGsettingsBase + ".custom-keybinding:" + gnomeCustomKBPath

	if err := gsettingsSet(schemaPath, "name", gnomeShortcutName); err != nil {
		return err
	}

	if err := gsettingsSet(schemaPath, "command", dbusCmd); err != nil {
		return err
	}

	if err := gsettingsSet(schemaPath, "binding", trigger); err != nil {
		return err
	}

	log.Printf("Hotkey GNOME: Registered shortcut with binding %s", trigger)

	return nil
}

// Unregister removes the GNOME shortcut and releases the D-Bus service.
func (g *gnomeBackend) Unregister() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		return nil
	}

	g.closed = true

	// Remove our entry from the custom-keybindings list
	paths, err := readCustomKeybindings()
	if err != nil {
		log.Printf("Hotkey GNOME: warning: failed to read custom keybindings: %v", err)
	} else {
		filtered := make([]string, 0, len(paths))

		for _, p := range paths {
			if p != gnomeCustomKBPath {
				filtered = append(filtered, p)
			}
		}

		if err := writeCustomKeybindings(filtered); err != nil {
			log.Printf("Hotkey GNOME: warning: failed to update custom keybindings: %v", err)
		}
	}

	// Reset shortcut properties
	schemaPath := gnomeGsettingsBase + ".custom-keybinding:" + gnomeCustomKBPath

	if err := gsettingsReset(schemaPath, "name"); err != nil {
		log.Printf("Hotkey GNOME: warning: %v", err)
	}

	if err := gsettingsReset(schemaPath, "command"); err != nil {
		log.Printf("Hotkey GNOME: warning: %v", err)
	}

	if err := gsettingsReset(schemaPath, "binding"); err != nil {
		log.Printf("Hotkey GNOME: warning: %v", err)
	}

	// Release D-Bus resources
	if g.conn != nil {
		if err := g.conn.Export(nil, dbus.ObjectPath(gnomeDBusPath), gnomeDBusIface); err != nil {
			log.Printf("Hotkey GNOME: warning: failed to unexport: %v", err)
		}

		if _, err := g.conn.ReleaseName(gnomeDBusName); err != nil {
			log.Printf("Hotkey GNOME: warning: failed to release D-Bus name: %v", err)
		}

		if err := g.conn.Close(); err != nil {
			return fmt.Errorf("failed to close D-Bus connection: %w", err)
		}
	}

	log.Println("Hotkey GNOME: Shortcut and D-Bus service unregistered")

	return nil
}

// Keydown returns the channel that receives hotkey activation events.
func (g *gnomeBackend) Keydown() <-chan struct{} {
	return g.keyChan
}

// buildGnomeTrigger converts modifier strings and key to GTK accelerator format.
// Example: ["Ctrl", "Shift"], "H" → "<Control><Shift>h"
func buildGnomeTrigger(modStrs []string, keyStr string) string {
	var b strings.Builder

	for _, mod := range modStrs {
		switch mod {
		case Ctrl:
			b.WriteString("<Control>")
		case Shift:
			b.WriteString("<Shift>")
		case Alt:
			b.WriteString("<Alt>")
		case Win, Cmd:
			b.WriteString("<Super>")
		default:
			b.WriteString("<")
			b.WriteString(mod)
			b.WriteString(">")
		}
	}

	b.WriteString(strings.ToLower(keyStr))

	return b.String()
}

// readCustomKeybindings reads the GNOME custom-keybindings gsettings array.
func readCustomKeybindings() ([]string, error) {
	out, err := exec.CommandContext(context.Background(), "gsettings", "get", gnomeGsettingsBase, "custom-keybindings").Output() //nolint:gosec // arguments are internally constructed
	if err != nil {
		return nil, fmt.Errorf("gsettings get custom-keybindings failed: %w", err)
	}

	return parseGsettingsArray(strings.TrimSpace(string(out))), nil
}

// writeCustomKeybindings writes the GNOME custom-keybindings gsettings array.
func writeCustomKeybindings(paths []string) error {
	var val string
	if len(paths) == 0 {
		val = gsettingsEmptyArray
	} else {
		quoted := make([]string, 0, len(paths))

		for _, p := range paths {
			if strings.ContainsAny(p, "'\\\n") {
				// Skip malformed entries that would break gsettings quoting.
				log.Printf("Hotkey GNOME: warning: skipping malformed keybinding path: %q", p)
				continue
			}

			quoted = append(quoted, "'"+p+"'")
		}

		if len(quoted) == 0 {
			val = gsettingsEmptyArray
		} else {
			var b strings.Builder
			b.WriteByte('[')

			for i, q := range quoted {
				if i > 0 {
					b.WriteString(", ")
				}

				b.WriteString(q)
			}

			b.WriteByte(']')
			val = b.String()
		}
	}

	cmd := exec.CommandContext(context.Background(), "gsettings", "set", gnomeGsettingsBase, "custom-keybindings", val) //nolint:gosec // arguments are internally constructed

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gsettings set custom-keybindings failed: %w (output: %s)", err, out)
	}

	return nil
}

// parseGsettingsArray parses the gsettings array format.
// Handles: "@as []", "[]", "['path1', 'path2']"
func parseGsettingsArray(s string) []string {
	if s == gsettingsEmptyArray || s == "[]" {
		return nil
	}

	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")

	if s == "" {
		return nil
	}

	parts := strings.Split(s, ", ")
	result := make([]string, 0, len(parts))

	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, "'")

		if p != "" {
			result = append(result, p)
		}
	}

	return result
}

// gsettingsSet runs gsettings set with the given schema path, key, and value.
func gsettingsSet(schemaPath, key, value string) error {
	cmd := exec.CommandContext(context.Background(), "gsettings", "set", schemaPath, key, value) //nolint:gosec // arguments are internally constructed

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gsettings set %s %s failed: %w (output: %s)", schemaPath, key, err, out)
	}

	return nil
}

// gsettingsReset runs gsettings reset with the given schema path and key.
func gsettingsReset(schemaPath, key string) error {
	cmd := exec.CommandContext(context.Background(), "gsettings", "reset", schemaPath, key) //nolint:gosec // arguments are internally constructed

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gsettings reset %s %s failed: %w (output: %s)", schemaPath, key, err, out)
	}

	return nil
}
