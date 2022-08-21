package top

import (
	"fmt"
	"github.com/Ivlyth/process-bandwidth/logging"
	"github.com/Ivlyth/process-bandwidth/version"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var logger = logging.GetLogger()

var app *tview.Application
var infoView *tview.TextView
var pages *tview.Pages

func StartTop() error {

	pTable = initProcessTable()

	cTable = initConnectionTable()

	graphView = initGraphView()

	connectionPanel = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(cTable, 0, 1, false).
		AddItem(graphView, 0, 2, false)

	processPanel = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(pTable, 0, 2, true).
		AddItem(connectionPanel, 0, 3, false)

	infoView = tview.NewTextView().SetDynamicColors(true).SetWrap(false)

	filterPanel := initFilterPanel()

	mainPanel := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(filterPanel, 3, 0, false).
		AddItem(processPanel, 0, 1, true).
		AddItem(infoView, 1, 0, false)

	mainPanel.SetBorder(true).
		SetTitle(fmt.Sprintf(" Process Bandwidth %s ", version.VERSION))

	pages = tview.NewPages()

	optionsPanel := initOptionsPanel()

	pages.AddPage("main", mainPanel, true, true).
		AddPage("options", optionsPanel, true, false)

	app = tview.NewApplication()
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTAB {
			if pTable.HasFocus() {
				app.SetFocus(connectionPanel).SetFocus(cTable)
			} else {
				app.SetFocus(processPanel).SetFocus(pTable)
			}
		} else if event.Key() == tcell.KeyF1 {
			// showOrHideHelp()  // TODO
		} else if event.Key() == tcell.KeyF4 {
			showOrHideOptions()
		} else if event.Key() == tcell.KeyF5 {
			paused = !paused
			if paused {
				mainPanel.SetTitle(tview.Escape(fmt.Sprintf(" Process Bandwidth %s [paused] ", version.VERSION)))
			} else {
				mainPanel.SetTitle(fmt.Sprintf(" Process Bandwidth %s ", version.VERSION))
			}
		} else if event.Key() == tcell.KeyESC {
			return tcell.NewEventKey(tcell.KeyCtrlC, 'c', tcell.ModNone)
		}
		return event
	})

	go tick()

	if err := app.SetRoot(pages, true).EnableMouse(true).Run(); err != nil {
		return err
	}
	return nil
}
