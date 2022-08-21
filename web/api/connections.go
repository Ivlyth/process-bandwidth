package api

import (
	"fmt"
	"github.com/Ivlyth/process-bandwidth/engine"
	uni_filter "github.com/Ivlyth/uni-filter"
	"github.com/gin-gonic/gin"
	"net/http"
)

func viewConnections(c *gin.Context) {
	connectionFilterStr, _ := c.GetQuery("cf") // connection filter
	var connectionFilter uni_filter.IExpr
	var err error
	if connectionFilterStr != "" {
		connectionFilter, err = uni_filter.Parse(connectionFilterStr)
		if err != nil {
			c.Writer.WriteString(fmt.Sprintf("parse connection filter failed: %s", err))
			return
		}
	}

	var connections []*engine.ConnectionInfo

	engine.ConnectionsMap.Range(func(inode string, bc *engine.ConnectionInfo) bool {
		if connectionFilter != nil && !connectionFilter.Match(bc) {
			return true
		}
		connections = append(connections, bc)
		return true
	})

	c.JSON(http.StatusOK, connections)
}
