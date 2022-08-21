package api

import (
	"fmt"
	"github.com/Ivlyth/process-bandwidth/config"
	"github.com/Ivlyth/process-bandwidth/engine"
	"github.com/gin-gonic/gin"
	"net/http"
)

// 用于查看内部的一些计数器
func overview(c *gin.Context) {
	// 各个 map 的大小
	var result = make(map[string]any, 20)
	var count int
	engine.ConnectionsMap.Range(func(inode string, c *engine.ConnectionInfo) bool {
		count++
		return true
	})
	result["ConnectionsMapCount"] = count

	count = 0

	//var pmap = make(map[uint32]any)
	ccount := 0
	engine.ProcessesMap.Range(func(pid uint32, bp *engine.Process) bool {
		count++
		bp.Connections.Range(func(fd uint32, bc *engine.Connection) bool {
			ccount++
			return true
		})
		//pmap[bp.Pid] = ccount
		return true
	})
	//result["ProcessesMap"] = pmap
	result["ProcessesMapCount"] = count
	result["ProcessesConnectionsMapCount"] = ccount

	//result["ringPoolCost"] = map[string]string{
	//	"get": engine.RingPool.GetCounter.String(),
	//	"put": engine.RingPool.PutCounter.String(),
	//}

	perfLost := ""
	if engine.PerfReaderLostProfileCounter.Count > 0 {
		perfLost = fmt.Sprintf("%.2f%%", float64(engine.PerfReaderLostProfileCounter.Count)*100.0/float64(engine.PerfReaderReadProfileCounter.Count))
	} else {
		perfLost = "0.00%"
	}
	result["perfReaderCost"] = map[string]string{
		"read": engine.PerfReaderReadProfileCounter.String(),
		"lost": engine.PerfReaderLostProfileCounter.String(),
		"rate": perfLost,
	}

	result["refreshConnectionsCost"] = engine.RefreshNetConnectionsProfileCounter.String()

	var worksCost = make(map[string]any, len(engine.EventProcessProfileCountersMap))
	for worker, pc := range engine.EventProcessProfileCountersMap {
		worksCost[worker] = pc.String()
	}
	result["workersCost"] = worksCost

	result["snapShotCost"] = engine.SnapShotProfileCounter.String()

	// config
	result["globalConfig"] = config.GlobalConfig
	c.JSON(http.StatusOK, result)
}
