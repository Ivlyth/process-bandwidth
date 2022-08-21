package asciigraph

import (
	"github.com/gdamore/tcell/v2"
	"strings"
)

// Option represents a configuration setting.
type Option interface {
	apply(c *config)
}

// config holds various graph options
type config struct {
	Width, Height int
	Offset        int
	Caption       string
	Precision     uint
	CaptionColor  AnsiColor
	AxisColor     AnsiColor
	LabelColor    AnsiColor
	SeriesColors  []AnsiColor
}

// An optionFunc applies an option.
type optionFunc func(*config)

// apply implements the Option interface.
func (of optionFunc) apply(c *config) { of(c) }

func configure(defaults config, options []Option) *config {
	for _, o := range options {
		o.apply(&defaults)
	}
	return &defaults
}

// Width sets the graphs width. By default, the width of the graph is
// determined by the number of data points. If the value given is a
// positive number, the data points are interpolated on the x axis.
// Values <= 0 reset the width to the default value.
func Width(w int) Option {
	return optionFunc(func(c *config) {
		if w > 0 {
			c.Width = w
		} else {
			c.Width = 0
		}
	})
}

// Height sets the graphs height.
func Height(h int) Option {
	return optionFunc(func(c *config) {
		if h > 0 {
			c.Height = h
		} else {
			c.Height = 0
		}
	})
}

// Offset sets the graphs offset.
func Offset(o int) Option {
	return optionFunc(func(c *config) { c.Offset = o })
}

// Precision sets the graphs precision.
func Precision(p uint) Option {
	return optionFunc(func(c *config) { c.Precision = p })
}

// Caption sets the graphs caption.
func Caption(caption string) Option {
	return optionFunc(func(c *config) {
		c.Caption = strings.TrimSpace(caption)
	})
}

// CaptionColor sets the caption color.
func CaptionColor(color tcell.Color) Option {
	return optionFunc(func(c *config) {
		c.CaptionColor = AnsiColor(color)
	})
}

// AxisColor sets the axis color.
func AxisColor(color tcell.Color) Option {
	return optionFunc(func(c *config) {
		c.AxisColor = AnsiColor(color)
	})
}

// LabelColor sets the axis label color.
func LabelColor(color tcell.Color) Option {
	return optionFunc(func(c *config) {
		c.LabelColor = AnsiColor(color)
	})
}

// SeriesColors sets the series colors.
func SeriesColors(colors ...tcell.Color) Option {
	var aColors = make([]AnsiColor, len(colors))
	for i, _ := range colors {
		aColors[i] = AnsiColor(colors[i])
	}
	return optionFunc(func(c *config) {
		c.SeriesColors = aColors
	})
}
