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
func SetupMenu(windowSignalChan chan<- string, shouldQuit chan<- bool) {
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
		mShow := systray.AddMenuItem(ui.TrayShowWindowTitle, ui.TrayShowWindowTooltip)
		mHide := systray.AddMenuItem(ui.TrayHideWindowTitle, ui.TrayHideWindowTooltip)

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
