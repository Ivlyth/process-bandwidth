package web

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/Ivlyth/process-bandwidth/internal/model"
	"github.com/Ivlyth/process-bandwidth/internal/store"
)

// pbmonCollector implements prometheus.Collector.
// It reads from the store on every Collect() call (i.e., on each scrape).
type pbmonCollector struct {
	store *store.Store

	netRxTotal   *prometheus.Desc
	netTxTotal   *prometheus.Desc
	fileRdTotal  *prometheus.Desc
	fileWrTotal  *prometheus.Desc
	netRxRate    *prometheus.Desc
	netTxRate    *prometheus.Desc
	fileRdRate   *prometheus.Desc
	fileWrRate   *prometheus.Desc
	connRxTotal  *prometheus.Desc
	connTxTotal  *prometheus.Desc
	activeProcs  *prometheus.Desc
	activeConns  *prometheus.Desc
	evDropped    *prometheus.Desc
	scrapeTime   *prometheus.Desc
}

func newPbmonCollector(s *store.Store) *pbmonCollector {
	const ns = "pbmon"
	labels := []string{"pid", "name"}
	connLabels := []string{"pid", "name", "protocol", "local", "remote"}

	return &pbmonCollector{
		store: s,

		netRxTotal:  prometheus.NewDesc(ns+"_process_net_rx_bytes_total", "Total network bytes received by process", labels, nil),
		netTxTotal:  prometheus.NewDesc(ns+"_process_net_tx_bytes_total", "Total network bytes sent by process", labels, nil),
		fileRdTotal: prometheus.NewDesc(ns+"_process_file_read_bytes_total", "Total file bytes read by process", labels, nil),
		fileWrTotal: prometheus.NewDesc(ns+"_process_file_write_bytes_total", "Total file bytes written by process", labels, nil),
		netRxRate:   prometheus.NewDesc(ns+"_process_net_rx_rate_bps", "Current network receive rate (bytes/s)", labels, nil),
		netTxRate:   prometheus.NewDesc(ns+"_process_net_tx_rate_bps", "Current network transmit rate (bytes/s)", labels, nil),
		fileRdRate:  prometheus.NewDesc(ns+"_process_file_read_rate_bps", "Current file read rate (bytes/s)", labels, nil),
		fileWrRate:  prometheus.NewDesc(ns+"_process_file_write_rate_bps", "Current file write rate (bytes/s)", labels, nil),
		connRxTotal: prometheus.NewDesc(ns+"_connection_rx_bytes_total", "Total bytes received on connection", connLabels, nil),
		connTxTotal: prometheus.NewDesc(ns+"_connection_tx_bytes_total", "Total bytes sent on connection", connLabels, nil),
		activeProcs: prometheus.NewDesc(ns+"_active_processes", "Number of active processes", nil, nil),
		activeConns: prometheus.NewDesc(ns+"_active_connections", "Number of active connections", nil, nil),
		evDropped:   prometheus.NewDesc(ns+"_ebpf_events_dropped_total", "Total eBPF events dropped due to ring buffer overflow", nil, nil),
		scrapeTime:  prometheus.NewDesc(ns+"_scrape_duration_seconds", "Time taken for Prometheus scrape", nil, nil),
	}
}

func (c *pbmonCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.netRxTotal
	ch <- c.netTxTotal
	ch <- c.fileRdTotal
	ch <- c.fileWrTotal
	ch <- c.netRxRate
	ch <- c.netTxRate
	ch <- c.fileRdRate
	ch <- c.fileWrRate
	ch <- c.connRxTotal
	ch <- c.connTxTotal
	ch <- c.activeProcs
	ch <- c.activeConns
	ch <- c.evDropped
	ch <- c.scrapeTime
}

func (c *pbmonCollector) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()
	procs := c.store.Processes()

	totalConns := 0
	for _, p := range procs {
		pidStr := strconv.FormatUint(uint64(p.PID), 10)
		name := p.Name()
		if name == "" {
			name = "?"
		}
		labels := []string{pidStr, name}

		ch <- prometheus.MustNewConstMetric(c.netRxTotal, prometheus.CounterValue, float64(p.Net.TotalRx()), labels...)
		ch <- prometheus.MustNewConstMetric(c.netTxTotal, prometheus.CounterValue, float64(p.Net.TotalTx()), labels...)
		ch <- prometheus.MustNewConstMetric(c.fileRdTotal, prometheus.CounterValue, float64(p.File.TotalRx()), labels...)
		ch <- prometheus.MustNewConstMetric(c.fileWrTotal, prometheus.CounterValue, float64(p.File.TotalTx()), labels...)
		ch <- prometheus.MustNewConstMetric(c.netRxRate, prometheus.GaugeValue, p.Net.RxRate(), labels...)
		ch <- prometheus.MustNewConstMetric(c.netTxRate, prometheus.GaugeValue, p.Net.TxRate(), labels...)
		ch <- prometheus.MustNewConstMetric(c.fileRdRate, prometheus.GaugeValue, p.File.RxRate(), labels...)
		ch <- prometheus.MustNewConstMetric(c.fileWrRate, prometheus.GaugeValue, p.File.TxRate(), labels...)

		p.EachConnection(func(conn *model.Connection) bool {
			totalConns++
			info := conn.Info()
			proto, local, remote := "-", "-", "-"
			if info != nil {
				proto = info.Protocol
				local = info.Local
				remote = info.Remote
			}
			connLabels := []string{pidStr, name, proto, local, remote}
			ch <- prometheus.MustNewConstMetric(c.connRxTotal, prometheus.CounterValue, float64(conn.IO.TotalRx()), connLabels...)
			ch <- prometheus.MustNewConstMetric(c.connTxTotal, prometheus.CounterValue, float64(conn.IO.TotalTx()), connLabels...)
			return true
		})
	}

	ch <- prometheus.MustNewConstMetric(c.activeProcs, prometheus.GaugeValue, float64(len(procs)))
	ch <- prometheus.MustNewConstMetric(c.activeConns, prometheus.GaugeValue, float64(totalConns))
	ch <- prometheus.MustNewConstMetric(c.evDropped, prometheus.CounterValue, float64(c.store.Dropped()))
	ch <- prometheus.MustNewConstMetric(c.scrapeTime, prometheus.GaugeValue, time.Since(start).Seconds())
}
