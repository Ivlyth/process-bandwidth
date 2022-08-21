package top

import (
	"fmt"
	"github.com/Ivlyth/process-bandwidth/engine"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sort"
	"time"
)

var lastUserScrollCTableAt time.Time // the last time user scroll up/down in the connection table
var connections Connections
var cTable *tview.Table
var selectedConnection *engine.Connection
var connectionPanel *tview.Flex

func initConnectionTableHeader() {

	colNames := []string{"No.", "FD", "Inode", "IfName", "Local Addr", "Remote Addr", "In", "Out", "Total"}
	color := tcell.ColorYellow

	for i, n := range colNames {
		if n == connectionsSortBy {
			if connectionsSortAsc {
				n = fmt.Sprintf("%s %s", n, string(Ascending))
			} else {
				n = fmt.Sprintf("%s %s", n, string(Descending))
			}
		}

		cTable.SetCell(0, i, tview.NewTableCell(n).
			SetExpansion(1).
			SetTextColor(color).
			SetAlign(tview.AlignCenter).
			SetSelectable(false).
			SetAttributes(tcell.AttrBold|tcell.AttrUnderline))
	}
	cTable.SetFixed(1, 1)
}

func initConnectionTable() *tview.Table {
	table := tview.NewTable().
		SetBorders(false).SetSelectable(true, false).SetSeparator(tview.Borders.Vertical)
	table.SetBorder(true).SetTitle(" Connection's Bandwidth ")
	table.SetEvaluateAllRows(true)

	table.SetSelectionChangedFunc(func(row, column int) {
		lastUserScrollCTableAt = time.Now()

		if row == 0 {
			if column == 6 { // in
				if connectionsSortBy == "In" { // 如果当前就是按照 in 进行排序, 那么就调整排序顺序
					connectionsSortAsc = !connectionsSortAsc
				} else { // 否则刚刚设置为使用该字段排序, 优先降序
					connectionsSortAsc = false
				}
				connectionsSortBy = "In"
			} else if column == 7 { // out
				if connectionsSortBy == "Out" { // 如果当前就是按照 out 进行排序, 那么就调整排序顺序
					connectionsSortAsc = !connectionsSortAsc
				} else { // 否则刚刚设置为使用该字段排序, 优先降序
					connectionsSortAsc = false
				}
				connectionsSortBy = "Out"
			} else if column == 8 { // total
				if connectionsSortBy == "Total" { // 如果当前就是按照 out 进行排序, 那么就调整排序顺序
					connectionsSortAsc = !connectionsSortAsc
				} else { // 否则刚刚设置为使用该字段排序, 优先降序
					connectionsSortAsc = false
				}
				connectionsSortBy = "Total"
			}
			if column >= 6 && column <= 8 {
				refreshConnectionTable()
			}
		}
	})

	//  set table header
	table.SetSelectedFunc(func(row int, column int) {
		v, ok := table.GetCell(row, 0).GetReference().(*engine.Connection)
		if ok {
			logger.Debugf("select connection inside cTable at row:%d, col:%d, connection fd is %d\n", row, column, v.FD)
			selectedConnection = v
		} else {
			logger.Debugf("select connection inside cTable at row:%d, col:%d, but connection is nil\n", row, column)
			selectedConnection = nil
		}

		refreshConnectionTable()
		refreshGraphPanel()
	})

	return table
}

func refreshConnectionTable() {
	cTable.Clear()
	cTable.SetTitle(" Connection's Bandwidth ")

	var pid uint32 = 0

	if selectedProcess != nil {
		connections = selectedProcess.GetConnections()
		pid = selectedProcess.Pid
	} else if len(processes) > 0 {
		connections = processes[0].GetConnections()
		pid = processes[0].Pid
	} else {
		connections = connections[:0]
	}

	nConnections := Connections{}
	for _, c := range connections {
		s := c.LastSnapshot()
		if isUnder(s, notShowUnderThis[notShowIdx].Limit) {
			continue
		}
		if filterForConnections && filterExpr != nil {
			if !filterExpr.Match(c) {
				continue
			}
		}
		nConnections = append(nConnections, c)
	}

	if pid > 0 {
		cTable.SetTitle(fmt.Sprintf(" Connections for PID %d (%d/%d) ", pid, len(nConnections), len(connections)))
	}

	if len(nConnections) > 0 {
		sort.Sort(nConnections)
		initConnectionTableHeader()

		selectedConnectionStillExists := false

		for i, _ := range nConnections {
			bc := nConnections[i]
			s := bc.LastSnapshot()
			updateConnectionTableColumns(i+1, bc,
				fmt.Sprintf("%4d", i+1),
				fmt.Sprintf("%d", bc.FD),
				bc.ConnectionInfo.Inode,
				bc.IfName,
				fmt.Sprintf("%s:%d", bc.ConnectionInfo.LocalIP, bc.ConnectionInfo.LocalPort),
				fmt.Sprintf("%s:%d", bc.ConnectionInfo.RemoteIP, bc.ConnectionInfo.RemotePort),
				s.IncomingRateAutobS(),
				s.OutgoingRateAutobS(),
				s.TotalRateAutobS())

			if selectedConnection != nil && selectedConnection.FD == bc.FD && selectedConnection.INode == bc.INode {
				selectedConnectionStillExists = true
				cTable.Select(i+1, 0)
			}
		}

		if !selectedConnectionStillExists && selectedConnection != nil {
			selectedConnection = nil
			cTable.ScrollToBeginning()
		}
	}
}

func updateConnectionTableColumns(row int, ref any, colValues ...string) {
	color := tcell.ColorDarkCyan
	selected := false
	for i, n := range colValues {
		if i > 0 {
			selected = true
			color = tcell.ColorWhite
		}

		cTable.SetCell(row, i, cTable.GetCell(row, i).SetExpansion(1).SetReference(ref).SetText(n).SetTextColor(color).SetAlign(tview.AlignCenter).SetSelectable(selected))
	}
}
