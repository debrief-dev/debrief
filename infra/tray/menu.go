package tray

import (
	"log"
	"sync"

	"fyne.io/systray"

	"github.com/debrief-dev/debrief/infra/platform"
	"github.com/debrief-dev/debrief/ui"
)

// MenuHandlers contains the channels for menu interactions
type MenuHandlers struct {
	WindowSignal chan<- string
	ShouldQuit   chan<- bool
}

var (
	handlers  *MenuHandlers
	setupOnce sync.Once
)

// SetupMenu initializes the system tray menu
func SetupMenu(windowSignalChan chan<- string, shouldQuit chan<- bool, hotkeyHint string) {
	setupOnce.Do(func() {
		handlers = &MenuHandlers{
			WindowSignal: windowSignalChan,
			ShouldQuit:   shouldQuit,
		}

		if platform.IsMacOS() {
			// Template icon adapts to macOS light/dark menu bar theme
			systray.SetTemplateIcon(GetIcon(), GetIcon())
		} else {
			systray.SetIcon(GetIcon())
			systray.SetTitle(ui.WindowTitle)
		}

		systray.SetTooltip(ui.WindowTitle)

		// Menu items
		showTitle := ui.TrayShowWindowTitle
		hideTitle := ui.TrayHideWindowTitle

		if hotkeyHint != "" {
			suffix := " (" + hotkeyHint + ")"
			showTitle += suffix
			hideTitle += suffix
		}

		mShow := systray.AddMenuItem(showTitle, ui.TrayShowWindowTooltip)
		mHide := systray.AddMenuItem(hideTitle, ui.TrayHideWindowTooltip)

		systray.AddSeparator()

		mQuit := systray.AddMenuItem(ui.TrayQuitTitle, ui.TrayQuitTooltip)

		// Handle menu clicks in a goroutine
		go func() {
			for {
				select {
				case <-mShow.ClickedCh:
					select {
					case handlers.WindowSignal <- "show":
					default:
					}

				case <-mHide.ClickedCh:
					select {
					case handlers.WindowSignal <- "hide":
					default:
					}

				case <-mQuit.ClickedCh:
					handlers.ShouldQuit <- true

					systray.Quit()
				}
			}
		}()
	})
}

// OnExit is called when the tray is exiting
func OnExit() {
	log.Println("Tray: Exiting")
}
