package font

import (
	_ "embed"
	"sync"

	"gioui.org/font"
	"gioui.org/font/opentype"
)

//go:embed FiraCode-Regular.ttf
var writerRegularTTF []byte

var (
	collection []font.FontFace
	once       sync.Once
)

// Collection returns the Writer font faces for use with text shaping.
func Collection() []font.FontFace {
	once.Do(func() {
		faces, err := opentype.ParseCollection(writerRegularTTF)
		if err != nil {
			panic("failed to parse font: " + err.Error())
		}

		collection = append(collection, faces...)
	})

	return collection
}
