package api

import (
	"fmt"
	"github.com/Ivlyth/process-bandwidth/engine"
	uni_filter "github.com/Ivlyth/uni-filter"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

type Connection struct {
	FD             uint32                 `json:"FD,omitempty"`
	FDType         uint8                  `json:"FDType"`
	CreatedAt      time.Time              `json:"CreatedAt,omitempty"`
	LastUpdateAt   time.Time              `json:"LastUpdateAt,omitempty"`
	Life           string                 `json:"Life,omitempty"`
	Fresh          string                 `json:"Fresh,omitempty"`
	ConnectionInfo *engine.ConnectionInfo `json:"ConnectionInfo,omitempty"`
	Snapshots      []engine.RWSnapshot    `json:"Snapshots,omitempty"`
}

type Process struct {
	Name         string              `json:"Name,omitempty"`
	Pid          uint32              `json:"Pid,omitempty"`
	CreatedAt    time.Time           `json:"CreatedAt,omitempty"`
	LastUpdateAt time.Time           `json:"LastUpdateAt,omitempty"`
	Life         string              `json:"Life,omitempty"`
	Fresh        string              `json:"Fresh,omitempty"`
	Cmdline      string              `json:"Cmdline,omitempty"`
	Snapshots    []engine.RWSnapshot `json:"Snapshots,omitempty"`
	Connections  []Connection        `json:"Connections"`
}

func viewProcesses(c *gin.Context) {
	_, withHis := c.GetQuery("his")
	_, anyConnection := c.GetQuery("ac")

	processFilterStr, _ := c.GetQuery("pf")    // process filter
	connectionFilterStr, _ := c.GetQuery("cf") // connection filter

	var err error

	var processFilter, connectionFilter uni_filter.IExpr

	if processFilterStr != "" {
		processFilter, err = uni_filter.Parse(processFilterStr)
		if err != nil {
			c.Writer.WriteString(fmt.Sprintf("parse process filter failed: %s", err))
			return
		}
	}

	if connectionFilterStr != "" {
		connectionFilter, err = uni_filter.Parse(connectionFilterStr)
		if err != nil {
			c.Writer.WriteString(fmt.Sprintf("parse connection filter failed: %s", err))
			return
		}
	}

	var results []Process

	engine.ProcessesMap.Range(func(pid uint32, bp *engine.Process) bool {

		p := Process{
			Name:         bp.Name,
			Pid:          bp.Pid,
			Cmdline:      bp.Cmdline,
			CreatedAt:    bp.CreatedAt,
			LastUpdateAt: bp.LastUpdateAt,
			Life:         fmt.Sprintf("%s", bp.LastUpdateAt.Sub(bp.CreatedAt)),
			Fresh:        fmt.Sprintf("%s", time.Now().Sub(bp.LastUpdateAt)),
		}
		if processFilter != nil && !processFilter.Match(p) {
			return true
		}

		var conns []Connection
		bp.Connections.Range(func(fd uint32, bc *engine.Connection) bool {
			if !anyConnection && bc.ShouldSkip() {
				return true
			}

			conn := Connection{
				FD:             bc.FD,
				FDType:         uint8(bc.FDType),
				ConnectionInfo: bc.ConnectionInfo,
				CreatedAt:      bc.CreatedAt,
				LastUpdateAt:   bc.LastUpdateAt,
				Life:           fmt.Sprintf("%s", bc.LastUpdateAt.Sub(bc.CreatedAt)),
				Fresh:          fmt.Sprintf("%s", time.Now().Sub(bc.LastUpdateAt)),
			}
			if connectionFilter != nil && !connectionFilter.Match(c) {
				return true
			}
			if withHis {
				conn.Snapshots = bc.Histories()
			}
			conns = append(conns, conn)
			return true
		})

		p.Connections = conns
		if withHis {
			p.Snapshots = bp.Histories()
		}

		results = append(results, p)

		return true
	})

	c.JSON(http.StatusOK, results)
}
