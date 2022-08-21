package top

import (
	"time"
)

var ticker = time.Tick(time.Duration(tickInterval) * time.Second)

func tick() {
	for {
		select {
		case <-ticker:
			if paused {
				continue
			}
			refreshAll()
			app.Draw()
		}
	}
}

func refreshAll() {
	if time.Now().Sub(lastUserScrollPTableAt) > 500*time.Millisecond {
		refreshProcessTable()
	}
	if time.Now().Sub(lastUserScrollCTableAt) > 500*time.Millisecond {
		refreshConnectionTable()
	}
	refreshGraphPanel()
}
