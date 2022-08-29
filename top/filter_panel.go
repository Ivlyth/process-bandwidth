package top

import (
	uni_filter "github.com/Ivlyth/uni-filter"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var filterStr = "!Name = sshd" // default
var filterExpr uni_filter.IExpr
var filterForProcess = true
var filterForConnections = false
var filterErr error

var filterPanel *tview.Flex
var filterInput *tview.InputField

func initFilterPanel() {
	filterInput = tview.NewInputField()
	filterExpr, filterErr = uni_filter.Parse(filterStr)
	filterInput.SetLabel("Filter: ").SetText(filterStr /*default filter*/).SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter {
			filterStr = filterInput.GetText()
			logger.Debugf("enter key pressed, filter text is: %s\n", filterStr)
			filterExpr, filterErr = uni_filter.Parse(filterStr)
			if filterErr != nil {
				filterExpr = nil
			}
			refreshAll()
		}
		return event
	})

	filterProcess := tview.NewCheckbox().SetLabel("Process: ").SetChecked(filterForProcess).SetChangedFunc(func(checked bool) {
		filterForProcess = checked
	})
	filterConnections := tview.NewCheckbox().SetLabel("Connes: ").SetChecked(filterForConnections).SetChangedFunc(func(checked bool) {
		filterForConnections = checked
	})
	//applyButton := tview.NewButton("<Apply>")
	//recentButton := tview.NewButton("<Recent>")

	filterPanel = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(filterInput, 0, 40, false).
		AddItem(nil, 0, 1, false).
		AddItem(filterProcess, 0, 5, false).
		AddItem(nil, 0, 1, false).
		AddItem(filterConnections, 0, 5, false)
	filterPanel.SetBorder(true)
}
