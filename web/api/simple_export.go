package api

import (
	"github.com/Ivlyth/process-bandwidth/engine"
	"github.com/gin-gonic/gin"
	"net/http"
)

func simpleView(c *gin.Context) {

	engine.SnapshotLock.Lock()
	defer engine.SnapshotLock.Unlock()

	rs := make(map[uint32][2]float64, 40)

	var s engine.RWSnapshot

	engine.ProcessesMap.Range(func(pid uint32, bp *engine.Process) bool {
		s = bp.LastSnapshot()
		in := s.IncomingRatebps()
		out := s.OutgoingRatebps()
		rs[bp.Pid] = [2]float64{in, out}
		return true
	})

	c.JSON(http.StatusOK, rs)
}
