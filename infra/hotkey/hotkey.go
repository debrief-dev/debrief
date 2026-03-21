package hotkey

import (
	"fmt"
	"log"
	"sync"
	"unicode/utf8"

	hk "golang.design/x/hotkey"
)

// Modifier name constants (shared with ui package for hotkey configuration).
const (
	Ctrl  = "Ctrl"
	Shift = "Shift"
	Alt   = "Alt"
	Win   = "Win"
	Cmd   = "Cmd"
)

type (
	Modifier = hk.Modifier
	Key      = hk.Key
)

// backend abstracts a global hotkey registration mechanism.
// The native backend wraps *hk.Hotkey; the portal backend uses D-Bus.
type backend interface {
	Register() error
	Unregister() error
	Keydown() <-chan struct{}
}

// Manager handles global hotkey registration.
// All exported methods are safe for concurrent use.
type Manager struct {
	mu         sync.Mutex
	b          backend
	signal     chan<- string
	done       chan struct{} // closed to stop the current listen goroutine
	registered bool
}

// NewManager creates a new hotkey manager
func NewManager(windowSignalChan chan<- string) *Manager {
	return &Manager{
		signal: windowSignalChan,
	}
}

// listen waits for hotkey events and sends toggle signals.
// It returns when done is closed or the backend channel is closed.
func (m *Manager) listen(keydown <-chan struct{}, done <-chan struct{}) {
	log.Println("Hotkey listener started")

	for {
		select {
		case <-done:
			log.Println("Hotkey listener stopped")
			return
		case _, ok := <-keydown:
			if !ok {
				log.Println("Hotkey keydown channel closed")
				return
			}

			log.Println("Hotkey pressed")

			select {
			case m.signal <- "toggle":
			default:
				// Signal channel full, drop duplicate.
			}
		}
	}
}

// stopListener cancels the running listen goroutine, if any.
// Caller must hold m.mu.
func (m *Manager) stopListener() {
	if m.done != nil {
		close(m.done)
		m.done = nil
	}
}

// StringToModifier converts string to hk.Modifier
func StringToModifier(s string) (hk.Modifier, error) {
	switch s {
	case Ctrl:
		return hk.ModCtrl, nil
	case Shift:
		return hk.ModShift, nil
	case Alt:
		return modAlt, nil
	case Win, Cmd:
		// "Cmd" on macOS maps to ModCmd, "Win" on Windows maps to ModWin,
		// on Linux maps to Mod4 (Super key)
		return modSuper, nil
	default:
		return 0, fmt.Errorf("unknown modifier: %s", s)
	}
}

// StringToKey converts string to hk.Key
func StringToKey(s string) (hk.Key, error) {
	// Single character keys (A-Z, 0-9)
	if utf8.RuneCountInString(s) == 1 {
		r, _ := utf8.DecodeRuneInString(s)

		if r >= 'A' && r <= 'Z' {
			return letterKeys[r-'A'], nil
		}

		if r >= '0' && r <= '9' {
			return digitKeys[r-'0'], nil
		}
	}

	// Special keys
	if s == "Space" {
		return hk.KeySpace, nil
	}

	return 0, fmt.Errorf("unknown key: %s", s)
}

// ConvertStrings converts modifier name strings and a key name string
// to their typed equivalents. Returns an error on the first unknown value.
func ConvertStrings(modNames []string, keyName string) ([]Modifier, Key, error) {
	mods := make([]Modifier, 0, len(modNames))

	for _, name := range modNames {
		mod, err := StringToModifier(name)
		if err != nil {
			return nil, 0, err
		}

		mods = append(mods, mod)
	}

	key, err := StringToKey(keyName)
	if err != nil {
		return nil, 0, err
	}

	return mods, key, nil
}

// UpdateHotkey unregisters current hotkey and registers new one.
// modStrs and keyStr are the string forms used by the Wayland portal backend.
func (m *Manager) UpdateHotkey(mods []hk.Modifier, key hk.Key, modStrs []string, keyStr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Unregister existing hotkey if registered
	if m.registered {
		log.Println("Unregistering previous hotkey before updating")

		m.stopListener()

		if err := m.b.Unregister(); err != nil {
			return fmt.Errorf("failed to unregister old hotkey: %w", err)
		}

		m.registered = false
	}

	b := newBackend(mods, key, modStrs, keyStr)

	log.Printf("Registering hotkey: %v + %s", modStrs, keyStr)

	if err := b.Register(); err != nil {
		// Clean up any resources the backend acquired during construction
		// (e.g., portal backend opens a D-Bus connection in newPortalBackend).
		if unregErr := b.Unregister(); unregErr != nil {
			log.Printf("Failed to clean up failed hotkey backend: %v", unregErr)
		}

		log.Printf("Failed to register hotkey: %v", err)

		return fmt.Errorf("failed to register hotkey (%v + %s): %w", modStrs, keyStr, err)
	}

	m.b = b

	m.registered = true
	m.done = make(chan struct{})

	log.Println("Hotkey registered successfully")

	go m.listen(m.b.Keydown(), m.done)

	return nil
}
