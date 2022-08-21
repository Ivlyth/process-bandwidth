package engine

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/Ivlyth/process-bandwidth/pkg/sync"
	psnet "github.com/shirou/gopsutil/v3/net"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	TCP_STATE_ESTABLISHED = "ESTABLISHED"
	TCP_STATE_SYN_SENT    = "SYN_SENT"
	TCP_STATE_SYN_RECV    = "SYN_RECV"
	TCP_STATE_FIN_WAIT1   = "FIN_WAIT1"
	TCP_STATE_FIN_WAIT2   = "FIN_WAIT2"
	TCP_STATE_TIME_WAIT   = "TIME_WAIT"
	TCP_STATE_CLOSE       = "CLOSE"
	TCP_STATE_CLOSE_WAIT  = "CLOSE_WAIT"
	TCP_STATE_LAST_ACK    = "LAST_ACK"
	TCP_STATE_LISTEN      = "LISTEN"
	TCP_STATE_CLOSING     = "CLOSING"
)

var TCPStateMap = map[string]string{
	"01": TCP_STATE_ESTABLISHED,
	"02": TCP_STATE_SYN_SENT,
	"03": TCP_STATE_SYN_RECV,
	"04": TCP_STATE_FIN_WAIT1,
	"05": TCP_STATE_FIN_WAIT2,
	"06": TCP_STATE_TIME_WAIT,
	"07": TCP_STATE_CLOSE,
	"08": TCP_STATE_CLOSE_WAIT,
	"09": TCP_STATE_LAST_ACK,
	"0A": TCP_STATE_LISTEN,
	"0B": TCP_STATE_CLOSING,
}

type ConnectionInfo struct {
	Inode string
	PID   uint32
	FD    uint32

	// if tcp or udp
	LocalIP    string
	LocalPort  uint16
	RemoteIP   string
	RemotePort uint16
	IsIPv6     bool
	IsLocal    bool
	Status     string
}

type FileInfo struct {
	Path   string
	Device string //属于哪个设备
}

// Connection 会涉及
type Connection struct {
	FD     uint32
	FDType FDType
	INode  string

	IfName string

	ConnectionInfo *ConnectionInfo // for fdtype == FDTypeSocket

	RWEventCounter
}

func (c *Connection) ShouldSkip() bool {
	if c.FDType != FDTypeSocket || c.ConnectionInfo == nil || c.ConnectionInfo.IsLocal {
		return true
	}
	return false
}

var ConnectionsMap = &sync.Map[string, *ConnectionInfo]{}
var cLock sync.Mutex

var hasTCP6 bool
var hasUDP6 bool

func init() {
	_, err := os.Stat("/proc/net/tcp6")
	if err == nil {
		hasTCP6 = true
	}

	_, err = os.Stat("/proc/net/udp6")
	if err == nil {
		hasUDP6 = true
	}
}

func findConnectionInfoBy(pid, fd uint32, inode string) *ConnectionInfo {
	v, loaded := ConnectionsMap.Load(inode)
	if loaded {
		return v
	}
	c := refreshNetConnections(pid, fd, inode)
	return c
}

func refreshFromInet(file string, family, sockType int, currentPid, currentFD uint32, currentINode string) *ConnectionInfo {
	contents, err := ioutil.ReadFile(file)
	if err != nil {
		return nil // keep quiet
	}

	lines := bytes.Split(contents, []byte("\n"))

	var laddr, raddr, status, inode string
	var pid, fd uint32
	var v any
	var loaded bool
	var c *ConnectionInfo

	var sip, dip net.IP
	var sport, dport uint16

	var ret *ConnectionInfo

	// skip first line
	for _, line := range lines[1:] {
		l := strings.Fields(string(line))
		if len(l) < 10 {
			continue
		}
		// _, laddr, raddr, status, _, _, _, _, _, inode = line.split()[:10]
		laddr = l[1]
		raddr = l[2]
		status = l[3]
		inode = l[9]
		pid = 0
		fd = 0 // FIXME what if we want to account file-liked read/write. maybe we use uint32.MAX as a special mark

		if sockType == syscall.SOCK_STREAM {
			if status != "01" { // 01 - ESTABLISHED
				continue
			}
		}
		sip, sport, err = decodeAddress(family, laddr)
		if err != nil {
			continue
		}
		dip, dport, err = decodeAddress(family, raddr)
		if err != nil {
			continue
		}

		v, loaded = ConnectionsMap.LoadOrStore(inode, func() *ConnectionInfo {
			return &ConnectionInfo{
				Inode:      inode,
				PID:        pid,
				FD:         fd,
				LocalIP:    sip.String(),
				LocalPort:  sport,
				RemoteIP:   dip.String(),
				RemotePort: dport,
				IsIPv6:     family == syscall.AF_INET6,
				IsLocal:    sip.IsLoopback(),
				Status:     status,
			}
		})

		c = v.(*ConnectionInfo)
		if loaded { // check it
			if c.LocalPort != sport || c.RemotePort != dport { // just use sport / dport to check whether it's same
				//c.Inode = inode
				c.PID = pid
				c.FD = fd
				c.LocalIP = sip.String()
				c.LocalPort = sport
				c.RemoteIP = dip.String()
				c.RemotePort = dport
				c.IsIPv6 = family == syscall.AF_INET6
				c.IsLocal = sip.IsLoopback()
				c.Status = status
			}
		}

		if currentINode == inode {
			c.PID = currentPid
			c.FD = currentFD
			ret = c
		}
	}
	return ret
}

func refreshNetConnections(currentPid, currentFD uint32, currentINode string) *ConnectionInfo {
	cLock.Lock()
	defer cLock.Unlock()

	startTime := time.Now()
	defer func() {
		RefreshNetConnectionsProfileCounter.Inc(time.Now().Sub(startTime))
	}()

	v, loaded := ConnectionsMap.Load(currentINode)
	if loaded {
		return v
	}

	var c *ConnectionInfo

	c = refreshFromInet("/proc/net/tcp", syscall.AF_INET, syscall.SOCK_STREAM, currentPid, currentFD, currentINode)
	if c != nil {
		return c
	}

	if hasTCP6 {
		c = refreshFromInet("/proc/net/tcp6", syscall.AF_INET6, syscall.SOCK_STREAM, currentPid, currentFD, currentINode)
		if c != nil {
			return c
		}
	}

	c = refreshFromInet("/proc/net/udp", syscall.AF_INET, syscall.SOCK_DGRAM, currentPid, currentFD, currentINode)
	if c != nil {
		return c
	}

	if hasUDP6 {
		c = refreshFromInet("/proc/net/udp6", syscall.AF_INET6, syscall.SOCK_DGRAM, currentPid, currentFD, currentINode)
		if c != nil {
			return c
		}
	}
	return nil
}

func parseIPv6HexString(src []byte) (net.IP, error) {
	if len(src) != 16 {
		return nil, fmt.Errorf("invalid IPv6 string")
	}

	buf := make([]byte, 0, 16)
	for i := 0; i < len(src); i += 4 {
		r := psnet.Reverse(src[i : i+4])
		buf = append(buf, r...)
	}
	return buf, nil
}

func decodeAddress(family int, src string) (net.IP, uint16, error) {
	t := strings.Split(src, ":")
	if len(t) != 2 {
		return nil, 0, fmt.Errorf("does not contain port, %s", src)
	}
	addr := t[0]
	var port uint16
	v, err := strconv.ParseUint(t[1], 16, 16)
	port = uint16(v)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid port, %s", src)
	}
	decoded, err := hex.DecodeString(addr)
	if err != nil {
		return nil, 0, fmt.Errorf("decode error, %w", err)
	}
	var ip net.IP
	// Assumes this is little_endian
	if family == syscall.AF_INET {
		ip = psnet.Reverse(decoded)
	} else { // IPv6
		ip, err = parseIPv6HexString(decoded)
		if err != nil {
			return nil, 0, err
		}
	}
	return ip, port, nil
}

func getConnectionInfoFor(pid, tid, fd uint32) *ConnectionInfo {
	inode, err := os.Readlink(fmt.Sprintf("/proc/%d/fd/%d", tid, fd))
	if err != nil {
		return nil
	}
	if !strings.HasPrefix(inode, "socket:[") {
		return nil
	}
	l := len(inode)
	inode = inode[8 : l-1]

	return findConnectionInfoBy(pid, fd, inode)
}
