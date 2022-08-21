package asciigraph

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
)

var Default = AnsiColor(tcell.ColorWhite)

type AnsiColor tcell.Color

func (c AnsiColor) String() string {
	return convertColorToName(tcell.Color(c))
}

func convertColorToName(c tcell.Color) string {
	for name, color := range tcell.ColorNames {
		if color == c {
			return fmt.Sprintf("[%s:]", name)
		}
	}
	return "" // default
}
