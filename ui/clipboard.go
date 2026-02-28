package ui

import (
	"io"
	"log"
	"strings"

	"gioui.org/io/clipboard"

	appstate "github.com/debrief-dev/debrief/app"
)

// copyTextAndMinimize copies text to clipboard and minimizes the window
func copyTextAndMinimize(gtx C, app *appstate.State, text string) {
	log.Printf("Copying text to clipboard and hiding window: %s", text)

	gtx.Execute(clipboard.WriteCmd{
		Type: "text/plain",
		Data: io.NopCloser(strings.NewReader(text)),
	})

	if app.HideWindowFunc != nil {
		app.HideWindowFunc()
	}
}
