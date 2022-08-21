package top

import "github.com/Ivlyth/process-bandwidth/engine"

type Processes []*engine.Process

// https://unicode-table.com/en/sets/arrow-symbols/#up-arrows

const (
	Ascending  rune = '↑'
	Descending rune = '↓'
)

var processSortBy = "Total"
var processSortByAsc = false

var connectionsSortBy = "Total"
var connectionsSortAsc = false

func (p Processes) Len() int {
	return len(p)
}

func (p Processes) Less(i, j int) bool {
	si := p[i].LastSnapshot()
	sj := p[j].LastSnapshot()

	var siv, sjv float64

	switch processSortBy {
	case "In":
		siv = si.IncomingRatebps()
		sjv = sj.IncomingRatebps()
	case "Out":
		siv = si.OutgoingRatebps()
		sjv = sj.OutgoingRatebps()
	case "Total":
		siv = si.TotalRatebps()
		sjv = sj.TotalRatebps()
	}
	if siv == sjv {
		return p[i].Pid < p[j].Pid
	} else if processSortByAsc {
		return siv < sjv
	} else {
		return siv > sjv
	}
}

func (p Processes) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

type Connections []*engine.Connection

func (c Connections) Len() int {
	return len(c)
}

func (c Connections) Less(i, j int) bool {
	ci := c[i].LastSnapshot()
	cj := c[j].LastSnapshot()
	var civ, cjv float64

	switch connectionsSortBy {
	case "In":
		civ = ci.IncomingRatebps()
		cjv = cj.IncomingRatebps()
	case "Out":
		civ = ci.OutgoingRatebps()
		cjv = cj.OutgoingRatebps()
	case "Total":
		civ = ci.TotalRatebps()
		cjv = cj.TotalRatebps()
	}
	if civ == cjv {
		return c[i].FD < c[j].FD
	} else if connectionsSortAsc {
		return civ < cjv
	} else {
		return civ > cjv
	}
}

func (c Connections) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}
