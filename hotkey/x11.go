package hotkey

import (
	"fmt"
	"sync"

	hk "golang.design/x/hotkey"
)

// nativeBackend wraps *hk.Hotkey for platforms that use the native
// X11/Win32/Cocoa hotkey library behind the backend interface.
type nativeBackend struct {
	hk      *hk.Hotkey
	keyChan chan struct{}
	done    chan struct{}
	once    sync.Once
}

func newNativeBackend(mods []hk.Modifier, key hk.Key) *nativeBackend {
	return &nativeBackend{
		hk: hk.New(mods, key),
	}
}

func (b *nativeBackend) Register() error {
	if err := b.hk.Register(); err != nil {
		return fmt.Errorf("hotkey register: %w", err)
	}

	// Bridge hk.Keydown() (chan hk.Event, i.e. chan struct{}) to chan struct{}
	// so the Manager can use a uniform backend interface.
	b.keyChan = make(chan struct{})
	b.done = make(chan struct{})

	go func() {
		defer close(b.keyChan)

		for {
			select {
			case <-b.done:
				return
			case _, ok := <-b.hk.Keydown():
				if !ok {
					return
				}

				select {
				case b.keyChan <- struct{}{}:
				case <-b.done:
					return
				}
			}
		}
	}()

	return nil
}

func (b *nativeBackend) Unregister() error {
	b.once.Do(func() {
		close(b.done)
	})

	if err := b.hk.Unregister(); err != nil {
		return fmt.Errorf("hotkey unregister: %w", err)
	}

	return nil
}

func (b *nativeBackend) Keydown() <-chan struct{} {
	return b.keyChan
}
