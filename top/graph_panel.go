package top

import (
	"fmt"
	"github.com/Ivlyth/process-bandwidth/engine"
	"github.com/Ivlyth/process-bandwidth/pkg/asciigraph"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var graphView *tview.TextView
var histories []engine.RWSnapshot

func initGraphView() *tview.TextView {
	v := tview.NewTextView().SetDynamicColors(true).SetWrap(false)
	v.SetBorder(true).SetTitle(" bandwidth graph ")
	_, _ = v.Write([]byte("waiting for select process or connection"))

	return v
}

func refreshGraphPanel() {
	// graph for selected process / connection
	graphView.Clear()
	histories = histories[:]
	if selectedConnection != nil {
		histories = selectedConnection.Histories()
		graphView.SetTitle(fmt.Sprintf(" bandwidth graph for FD %d ", selectedConnection.FD))
	} else if selectedProcess != nil {
		histories = selectedProcess.Histories()
		graphView.SetTitle(fmt.Sprintf(" bandwidth graph for PID %d ", selectedProcess.Pid))
	} else if len(processes) > 0 {
		histories = processes[0].Histories()
		graphView.SetTitle(fmt.Sprintf(" bandwidth graph for PID %d ", processes[0].Pid))
	}

	if len(histories) > 0 {
		_, _, width, height := graphView.GetInnerRect()
		height -= 2
		width -= 2
		if height <= 4 {
			height = 0
			_, _ = graphView.Write([]byte("not enough height to draw the graph (at least 4)"))
		} else {
			var in, out []float64
			for _, h := range histories {
				in = append(in, h.IncomingRateMbps())
				out = append(out, h.OutgoingRateMbps())
			}
			p := asciigraph.PlotMany([][]float64{in, out}, asciigraph.Height(height), asciigraph.Width(width), asciigraph.Precision(2), asciigraph.SeriesColors(tcell.ColorNames[colorOptions[inColorIdx]], tcell.ColorNames[colorOptions[outColorIdx]]))
			_, _ = graphView.Write([]byte(p))
		}
	} else {
		_, _ = graphView.Write([]byte("no data for selected process or connection"))
	}
}
