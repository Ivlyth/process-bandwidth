package debug

import (
	"fmt"
	"github.com/Ivlyth/process-bandwidth/logging"
	"github.com/pkg/errors"
	"net/http"
	_ "net/http/pprof"
)

var logger = logging.GetLogger()

func StartProfileServer(port uint16, errChan chan<- error) {
	err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", port), nil)
	if err != nil {
		errChan <- errors.Wrap(err, "error when start pprof server")
		return
	}
	logger.Println(fmt.Sprintf("pprof http server listen on port %d", port))
	return
}
