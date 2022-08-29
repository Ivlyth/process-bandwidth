package top

import (
	"fmt"
	"github.com/Ivlyth/process-bandwidth/config"
	"github.com/Ivlyth/process-bandwidth/engine"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sort"
	"time"
)

var processes Processes
var lastUserScrollPTableAt time.Time // the last time user scroll up/down in the process table
var pTable *tview.Table
var selectedProcess *engine.Process
var processPanel *tview.Flex

func initProcessTableHeader() {

	colNames := []string{"No.", "Pid", "Name", "In", "Out", "Total"}
	color := tcell.ColorYellow

	for i, n := range colNames {
		if n == processSortBy {
			if processSortByAsc {
				n = fmt.Sprintf("%s %s", n, string(Ascending))
			} else {
				n = fmt.Sprintf("%s %s", n, string(Descending))
			}
		}

		pTable.SetCell(0, i, tview.NewTableCell(n).
			SetExpansion(1).
			SetTextColor(color).
			SetAlign(tview.AlignCenter).
			SetSelectable(false).
			SetAttributes(tcell.AttrBold|tcell.AttrUnderline))
	}
	pTable.SetFixed(1, 1)
}

func initProcessTable() {
	pTable = tview.NewTable().
		SetBorders(false).SetSelectable(true, false).SetSeparator(tview.Borders.Vertical)
	pTable.SetBorder(true).SetTitle(" Process's Bandwidth ")
	pTable.SetEvaluateAllRows(true)

	pTable.SetSelectionChangedFunc(func(row, column int) {
		lastUserScrollPTableAt = time.Now()
		v, ok := pTable.GetCell(row, 0).GetReference().(*engine.Process)
		if ok {
			infoView.Clear()
			_, _ = infoView.Write([]byte(v.Cmdline))
			if config.GlobalConfig.Debug {
				_, _ = infoView.Write([]byte(fmt.Sprintf(" (live for %s, update for %s)", time.Now().Sub(v.CreatedAt), time.Now().Sub(v.LastUpdateAt))))
			}
		}

		if row == 0 {
			if column == 3 { // in
				if processSortBy == "In" { // 如果当前就是按照 in 进行排序, 那么就调整排序顺序
					processSortByAsc = !processSortByAsc
				} else { // 否则刚刚设置为使用该字段排序, 优先降序
					processSortByAsc = false
				}
				processSortBy = "In"
			} else if column == 4 { // out
				if processSortBy == "Out" { // 如果当前就是按照 out 进行排序, 那么就调整排序顺序
					processSortByAsc = !processSortByAsc
				} else { // 否则刚刚设置为使用该字段排序, 优先降序
					processSortByAsc = false
				}
				processSortBy = "Out"
			} else if column == 5 { // total
				if processSortBy == "Total" { // 如果当前就是按照 out 进行排序, 那么就调整排序顺序
					processSortByAsc = !processSortByAsc
				} else { // 否则刚刚设置为使用该字段排序, 优先降序
					processSortByAsc = false
				}
				processSortBy = "Total"
			}
			if column >= 3 && column <= 5 {
				refreshProcessTable()
			}
		}
	})

	pTable.SetSelectedFunc(func(row int, column int) {
		logger.Debugf("ptable selected row:%d, column:%d\n", row, column)
		v, ok := pTable.GetCell(row, 0).GetReference().(*engine.Process)
		infoView.Clear()
		if ok {
			selectedProcess = v
			_, _ = infoView.Write([]byte(v.Cmdline))
			if config.GlobalConfig.Debug {
				_, _ = infoView.Write([]byte(fmt.Sprintf(" (live for %s, update for %s)", time.Now().Sub(v.CreatedAt), time.Now().Sub(v.LastUpdateAt))))
			}

			logger.Debugf("select process inside pTable at row:%d, col:%d, process pid is %d\n", row, column, v.Pid)
		} else {
			logger.Debugf("select process inside pTable at row:%d, col:%d, but process is nil\n", row, column)
			selectedProcess = nil
		}
		selectedConnection = nil

		refreshConnectionTable()
		refreshGraphPanel()
	})
}

func refreshProcessTable() {
	processes = Processes{}
	engine.ProcessesMap.Range(func(pid uint32, bp *engine.Process) bool {
		s := bp.LastSnapshot()
		if isUnder(s, notShowUnderThis[notShowIdx].Limit) {
			return true
		}
		if filterForProcess && filterExpr != nil {
			if !filterExpr.Match(bp) {
				return true
			}
		}
		processes = append(processes, bp)
		return true
	})

	if len(processes) > 0 {
		sort.Sort(processes)
	}

	pTable.Clear()
	initProcessTableHeader()

	selectedProcessStillExists := false

	for i, _ := range processes {
		bp := processes[i]
		s := bp.LastSnapshot()
		updateProcessTableColumns(i+1, bp,
			fmt.Sprintf("%4d", i+1),
			fmt.Sprintf("%d", bp.Pid),
			fmt.Sprintf("%s", bp.Name),
			s.IncomingRateAutobS(),
			s.OutgoingRateAutobS(),
			s.TotalRateAutobS())
		if selectedProcess != nil {
			if selectedProcess.Pid == bp.Pid {
				selectedProcessStillExists = true
				pTable.Select(i+1, 0)

				infoView.Clear()
				_, _ = infoView.Write([]byte(bp.Cmdline))
				if config.GlobalConfig.Debug {
					_, _ = infoView.Write([]byte(fmt.Sprintf(" (live for %s, update for %s)", time.Now().Sub(bp.CreatedAt), time.Now().Sub(bp.LastUpdateAt))))
				}
			}
		} else if i == 0 {
			pTable.Select(i+1, 0)

			infoView.Clear()
			_, _ = infoView.Write([]byte(bp.Cmdline))
			if config.GlobalConfig.Debug {
				_, _ = infoView.Write([]byte(fmt.Sprintf(" (live for %s, update for %s)", time.Now().Sub(bp.CreatedAt), time.Now().Sub(bp.LastUpdateAt))))
			}
		}
	}

	if !selectedProcessStillExists && selectedProcess != nil {
		selectedProcess = nil
		infoView.Clear()
		pTable.ScrollToBeginning()
	}
}

func updateProcessTableColumns(row int, ref any, colValues ...string) {
	color := tcell.ColorDarkCyan
	selected := false
	for i, n := range colValues {
		if i > 0 {
			selected = true
			color = tcell.ColorWhite
		}

		pTable.SetCell(row, i, pTable.GetCell(row, i).SetExpansion(1).SetReference(ref).SetText(n).SetTextColor(color).SetAlign(tview.AlignCenter).SetSelectable(selected))
	}
}
