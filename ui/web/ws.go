package web

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/Ivlyth/process-bandwidth/internal/model"
	"github.com/Ivlyth/process-bandwidth/internal/store"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // Allow all origins in development
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
}

// wsHub manages all WebSocket connections and broadcasts store updates.
type wsHub struct {
	store   *store.Store
	logger  *slog.Logger
	clients sync.Map // *websocket.Conn -> struct{}
}

func newWSHub(s *store.Store, logger *slog.Logger) *wsHub {
	return &wsHub{store: s, logger: logger}
}

// Run starts the broadcast loop. It sends a JSON snapshot to all clients
// every second, and exits when ctx is cancelled.
func (h *wsHub) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			// Close all remaining clients
			h.clients.Range(func(k, _ any) bool {
				k.(*websocket.Conn).Close()
				return true
			})
			return
		case <-ticker.C:
			data := h.buildSnapshot()
			h.broadcast(data)
		}
	}
}

func (h *wsHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Warn("WebSocket upgrade failed", "err", err)
		return
	}
	h.clients.Store(conn, struct{}{})

	// Drain incoming messages (we don't expect any, but need to for ping/pong)
	go func() {
		defer func() {
			h.clients.Delete(conn)
			conn.Close()
		}()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()
}

type wsProcess struct {
	PID         uint32  `json:"pid"`
	Name        string  `json:"name"`
	Cmdline     string  `json:"cmdline"`
	NetRxRate   float64 `json:"net_rx_rate"`
	NetTxRate   float64 `json:"net_tx_rate"`
	FileRdRate  float64 `json:"file_rd_rate"`
	FileWrRate  float64 `json:"file_wr_rate"`
	NetRxTotal  uint64  `json:"net_rx_total"`
	NetTxTotal  uint64  `json:"net_tx_total"`
	Connections int     `json:"connections"`
}

type wsSnapshot struct {
	Ts       int64       `json:"ts"`       // unix millis
	Dropped  uint64      `json:"dropped"`
	Processes []wsProcess `json:"processes"`
}

func (h *wsHub) buildSnapshot() []byte {
	procs := h.store.Processes()
	snap := wsSnapshot{
		Ts:        time.Now().UnixMilli(),
		Dropped:   h.store.Dropped(),
		Processes: make([]wsProcess, 0, len(procs)),
	}
	for _, p := range procs {
		snap.Processes = append(snap.Processes, wsProcess{
			PID:        p.PID,
			Name:       p.Name(),
			Cmdline:    p.Cmdline(),
			NetRxRate:  p.Net.RxRate(),
			NetTxRate:  p.Net.TxRate(),
			FileRdRate: p.File.RxRate(),
			FileWrRate: p.File.TxRate(),
			NetRxTotal: p.Net.TotalRx(),
			NetTxTotal: p.Net.TotalTx(),
			Connections: p.ConnectionCount(),
		})
	}
	data, _ := json.Marshal(snap)
	return data
}

func (h *wsHub) broadcast(data []byte) {
	h.clients.Range(func(k, _ any) bool {
		conn := k.(*websocket.Conn)
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			h.clients.Delete(conn)
			conn.Close()
		}
		return true
	})
}

// apiProcesses returns JSON for the REST /api/processes endpoint.
func apiProcesses(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		procs := s.Processes()
		type connResp struct {
			FD       uint32 `json:"fd"`
			Class    string `json:"class"`
			Protocol string `json:"protocol,omitempty"`
			Local    string `json:"local,omitempty"`
			Remote   string `json:"remote,omitempty"`
			RxRate   float64 `json:"rx_rate"`
			TxRate   float64 `json:"tx_rate"`
		}
		type procResp struct {
			PID         uint32     `json:"pid"`
			Name        string     `json:"name"`
			Cmdline     string     `json:"cmdline"`
			NetRxRate   float64    `json:"net_rx_rate"`
			NetTxRate   float64    `json:"net_tx_rate"`
			FileRdRate  float64    `json:"file_rd_rate"`
			FileWrRate  float64    `json:"file_wr_rate"`
			NetRxTotal  uint64     `json:"net_rx_total"`
			NetTxTotal  uint64     `json:"net_tx_total"`
			Connections []connResp `json:"connections,omitempty"`
		}

		result := make([]procResp, 0, len(procs))
		for _, p := range procs {
			pr := procResp{
				PID:        p.PID,
				Name:       p.Name(),
				Cmdline:    p.Cmdline(),
				NetRxRate:  p.Net.RxRate(),
				NetTxRate:  p.Net.TxRate(),
				FileRdRate: p.File.RxRate(),
				FileWrRate: p.File.TxRate(),
				NetRxTotal: p.Net.TotalRx(),
				NetTxTotal: p.Net.TotalTx(),
			}
			if r.URL.Query().Get("conns") == "1" {
				p.EachConnection(func(c *model.Connection) bool {
					cls := c.Class
					if rc := c.ResolvedClass(); rc != model.FDClassUnknown {
						cls = rc
					}
					cr := connResp{
						FD:    c.FD,
						Class: cls.String(),
					}
					if info := c.Info(); info != nil {
						cr.Protocol = info.Protocol
						cr.Local = info.Local
						cr.Remote = info.Remote
					}
					cr.RxRate = c.IO.RxRate()
					cr.TxRate = c.IO.TxRate()
					pr.Connections = append(pr.Connections, cr)
					return true
				})
			}
			result = append(result, pr)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// apiOverview returns a system-wide summary.
func apiOverview(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		procs := s.Processes()
		var totalNetRx, totalNetTx, totalFileRd, totalFileWr float64
		totalConns := 0
		for _, p := range procs {
			totalNetRx += p.Net.RxRate()
			totalNetTx += p.Net.TxRate()
			totalFileRd += p.File.RxRate()
			totalFileWr += p.File.TxRate()
			totalConns += p.ConnectionCount()
		}
		resp := map[string]any{
			"processes":        len(procs),
			"connections":      totalConns,
			"total_net_rx_bps": totalNetRx,
			"total_net_tx_bps": totalNetTx,
			"total_file_rd_bps": totalFileRd,
			"total_file_wr_bps": totalFileWr,
			"dropped_events":   s.Dropped(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// apiProcessDetail handles GET /api/processes/{pid} and returns per-process
// historical bandwidth samples for use by the web chart.
func apiProcessDetail(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pidStr := strings.TrimPrefix(r.URL.Path, "/api/processes/")
		pid64, err := strconv.ParseUint(pidStr, 10, 32)
		if err != nil {
			http.Error(w, "bad pid", http.StatusBadRequest)
			return
		}
		proc := s.Get(uint32(pid64))
		if proc == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		toRates := func(samples []model.IOSample, rx bool) []float64 {
			rates := make([]float64, len(samples))
			for i, s := range samples {
				if rx {
					rates[i] = s.RxRateBps()
				} else {
					rates[i] = s.TxRateBps()
				}
			}
			return rates
		}

		netHist := proc.Net.History()
		fileHist := proc.File.History()
		resp := map[string]any{
			"pid":             proc.PID,
			"name":            proc.Name(),
			"net_rx_history":  toRates(netHist, true),
			"net_tx_history":  toRates(netHist, false),
			"file_rd_history": toRates(fileHist, true),
			"file_wr_history": toRates(fileHist, false),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
