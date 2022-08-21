package top

import (
	"fmt"
	"github.com/Ivlyth/process-bandwidth/engine"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"strconv"
	"time"
)

func init() {
	var i int
	for name, _ := range tcell.ColorNames {
		colorOptions = append(colorOptions, name)
		if name == "yellow" {
			inColorIdx = i
		} else if name == "red" {
			outColorIdx = i
		}
		i++
	}

	for _, n := range notShowUnderThis {
		notShowOptions = append(notShowOptions, n.Desc)
	}
}

// paused indicates current in the paused state.
// in the paused state, we just skip every tick to not update the view
// but user can still scroll up/down to see each process / connection
// by press enter key, they even can see the latest graph for process or connection
var paused bool

// inColorIdx is color idx for in-series color
var inColorIdx int

// outColorIdx is color idx for out-series color
var outColorIdx int

// colorOptions is all available colors, init at runtime
var colorOptions []string

// tickInterval is the interval we update the view, in seconds
var tickInterval = 1

// isUnder return true when the given Snapshot's TotalRatebps value is under v
func isUnder(s engine.RWSnapshot, v float64) bool {
	return s.TotalRatebps() < v
}

type NotShowUnder struct {
	Desc  string
	Limit float64
}

var notShowUnderThis = []NotShowUnder{
	{
		"No Limit",
		0,
	},
	{
		" 10 Kbps",
		10 * 1024,
	},
	{
		" 50 Kbps",
		50 * 1024,
	},
	{
		"100 Kbps",
		100 * 1024,
	},
	{
		"500 Kbps",
		500 * 1024,
	},
	{
		"  1 Mbps",
		1 * 1024 * 1024,
	},
	{
		"  5 Mbps",
		5 * 1024 * 1024,
	},
	{
		" 10 Mbps",
		10 * 1024 * 1024,
	},
	{
		" 50 Mbps",
		50 * 1024 * 1024,
	},
}

// notShowIdx indicates current selection for not shown under setting
// default is 0, no limit
var notShowIdx int

// notShowOptions is all available options for not shown under setting, init at runtime
var notShowOptions []string

// Options used for change settings through `options panel`
type Options struct {
	tickIntervalS string
	inColorIdx    int
	outColorIdx   int
	notShowIdx    int
}

// pendingOptions used for when user open the settings window, to record what user selected, and apply when user click Save
var pendingOptions Options

// showingOptionsPage indicates whether current showing the options panel or not
var showingOptionsPage bool

func initOptionsPanel() *tview.Flex {
	optionsForm := tview.NewForm().
		AddInputField("Refresh Interval (secs)", fmt.Sprintf("%d", tickInterval), 20, func(textToCheck string, lastChar rune) bool {
			if lastChar >= '0' && lastChar <= '9' {
				return true
			}
			return false
		}, func(text string) {
			pendingOptions.tickIntervalS = text
		}).
		AddDropDown("In Series Color", colorOptions, inColorIdx, func(option string, optionIndex int) {
			pendingOptions.inColorIdx = optionIndex
		}).
		AddDropDown("Out Series Color", colorOptions, outColorIdx, func(option string, optionIndex int) {
			pendingOptions.outColorIdx = optionIndex
		}).
		AddDropDown("Not show under", notShowOptions, notShowIdx, func(option string, optionIndex int) {
			pendingOptions.notShowIdx = optionIndex
		}).
		AddButton("Save", func() {
			applyOptions()
			showOrHideOptions()
		}).
		AddButton("Cancel", func() {
			showOrHideOptions()
		})
	optionsForm.SetBorder(true).SetTitle(" Options ")

	optionsFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(nil, 0, 3, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(optionsForm, 13, 0, false).
			AddItem(nil, 0, 1, false), 0, 2, false).
		AddItem(nil, 0, 3, false)

	return optionsFlex
}

func applyOptions() {
	inColorIdx = pendingOptions.inColorIdx
	outColorIdx = pendingOptions.outColorIdx
	notShowIdx = pendingOptions.notShowIdx

	secs, err := strconv.Atoi(pendingOptions.tickIntervalS)
	if err == nil && secs != tickInterval {
		tickInterval = secs
		duration := time.Duration(tickInterval) * time.Second
		ticker = time.Tick(duration)
	}
}

func showOrHideOptions() {
	if showingOptionsPage {
		showingOptionsPage = false
		pages.ShowPage("main")
		pages.HidePage("options")
	} else {
		pendingOptions = Options{
			tickIntervalS: fmt.Sprintf("%d", tickInterval),
			inColorIdx:    inColorIdx,
			outColorIdx:   outColorIdx,
			notShowIdx:    notShowIdx,
		}
		showingOptionsPage = true
		pages.ShowPage("options")
		pages.HidePage("main")
	}
}
