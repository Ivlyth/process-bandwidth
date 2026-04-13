package collector

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Ivlyth/process-bandwidth/internal/model"
)

// NetResolver maintains a live cache of inode → ConnectionInfo by periodically
// parsing /proc/net/tcp, tcp6, udp, udp6, and unix.
//
// Hot path (handleIO): cache.Load(inode) – lock-free sync.Map read.
// Background: re-parse all files every cfg.NetRefreshInterval (default 100ms).
type NetResolver struct {
	cache       sync.Map // map[uint64]*model.ConnectionInfo
	refreshCh   chan struct{}
	interval    time.Duration
}

// NewNetResolver creates a NetResolver with the given refresh interval.
func NewNetResolver(interval time.Duration) *NetResolver {
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	return &NetResolver{
		refreshCh: make(chan struct{}, 1),
		interval:  interval,
	}
}

// Start begins the background refresh loop. It returns when ctx is cancelled.
func (r *NetResolver) Start(ctx context.Context) {
	go r.loop(ctx)
}

func (r *NetResolver) loop(ctx context.Context) {
	r.refresh() // immediate first refresh
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.refresh()
		case <-r.refreshCh:
			r.refresh()
		}
	}
}

// TriggerRefresh requests an immediate out-of-band refresh (non-blocking).
func (r *NetResolver) TriggerRefresh() {
	select {
	case r.refreshCh <- struct{}{}:
	default:
	}
}

// LookupByFD resolves a connection by reading /proc/tid/fd/fd symlink to
// get the socket inode, then looking it up in the cache.
// Returns nil if the FD is not a socket or the inode is not in the cache.
func (r *NetResolver) LookupByFD(pid, fd uint32) *model.ConnectionInfo {
	inode, ok := readSocketInode(pid, fd)
	if !ok {
		return nil
	}
	v, ok := r.cache.Load(inode)
	if !ok {
		// Trigger a refresh and return nil; next call may find it.
		r.TriggerRefresh()
		return nil
	}
	return v.(*model.ConnectionInfo)
}

// LookupByInode looks up a ConnectionInfo by inode number.
func (r *NetResolver) LookupByInode(inode uint64) *model.ConnectionInfo {
	v, ok := r.cache.Load(inode)
	if !ok {
		return nil
	}
	return v.(*model.ConnectionInfo)
}

// refresh parses all /proc/net/* files and updates the cache.
func (r *NetResolver) refresh() {
	r.parseTCP("/proc/net/tcp", "tcp4", false)
	r.parseTCP("/proc/net/tcp6", "tcp6", true)
	r.parseTCP("/proc/net/udp", "udp4", false)
	r.parseTCP("/proc/net/udp6", "udp6", true)
	r.parseUnix("/proc/net/unix")
}

// parseTCP parses /proc/net/tcp[6] or /proc/net/udp[6].
// Each line (after the header) has the format:
//
//	sl  local_address rem_address   st tx_queue rx_queue ...  inode
func (r *NetResolver) parseTCP(path, proto string, ipv6 bool) {
	f, err := os.Open(path)
	if err != nil {
		return // file may not exist (e.g. no IPv6)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Scan() // skip header line
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		// Fields: sl local_addr rem_addr status tx_queue rx_queue ... uid inode
		// We need: local_addr[1], rem_addr[2], inode[9]
		if len(fields) < 10 {
			continue
		}
		localStr := fields[1]
		remoteStr := fields[2]
		inodeStr := fields[9]

		inode, err := strconv.ParseUint(inodeStr, 10, 64)
		if err != nil {
			continue
		}

		local := parseHexAddr(localStr, ipv6)
		remote := parseHexAddr(remoteStr, ipv6)

		info := &model.ConnectionInfo{
			Inode:    inode,
			Local:    local,
			Remote:   remote,
			Protocol: proto,
		}
		r.cache.Store(inode, info)
	}
}

// parseUnix parses /proc/net/unix for Unix domain sockets.
// Fields: Num RefCount Protocol Flags Type St Inode Path
func (r *NetResolver) parseUnix(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Scan() // skip header
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}
		inodeStr := fields[6]
		inode, err := strconv.ParseUint(inodeStr, 10, 64)
		if err != nil {
			continue
		}
		socketPath := ""
		if len(fields) >= 8 {
			socketPath = fields[7]
		}
		info := &model.ConnectionInfo{
			Inode:    inode,
			Local:    socketPath,
			Remote:   "",
			Protocol: "unix",
		}
		r.cache.Store(inode, info)
	}
}

// parseHexAddr converts a hex "AABBCCDD:PPPP" (IPv4) or
// "AABBCCDDAABBCCDDAABBCCDDAABBCCDD:PPPP" (IPv6) to "ip:port" notation.
func parseHexAddr(s string, ipv6 bool) string {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return s
	}
	addrHex, portHex := parts[0], parts[1]

	port, err := strconv.ParseUint(portHex, 16, 16)
	if err != nil {
		return s
	}

	if ipv6 {
		b, err := hex.DecodeString(addrHex)
		if err != nil || len(b) != 16 {
			return s
		}
		// IPv6: stored as 4 groups of 4 bytes in little-endian order.
		// Reverse each 4-byte group.
		for i := 0; i < 16; i += 4 {
			b[i], b[i+3] = b[i+3], b[i]
			b[i+1], b[i+2] = b[i+2], b[i+1]
		}
		ip := net.IP(b)
		return fmt.Sprintf("[%s]:%d", ip.String(), port)
	}

	// IPv4: 4-byte hex, little-endian (e.g. "0100007F" = 127.0.0.1)
	b, err := hex.DecodeString(addrHex)
	if err != nil || len(b) != 4 {
		return s
	}
	ip := net.IPv4(b[3], b[2], b[1], b[0])
	return fmt.Sprintf("%s:%d", ip.String(), port)
}

// readSocketInode reads /proc/pid/fd/fd as a symlink and extracts the inode
// from a "socket:[12345]" value.
// Returns (inode, true) on success, (0, false) otherwise.
func readSocketInode(pid, fd uint32) (uint64, bool) {
	linkPath := "/proc/" + uitoa(pid) + "/fd/" + uitoa(fd)
	target, err := os.Readlink(linkPath)
	if err != nil {
		return 0, false
	}
	// target has the form "socket:[12345]"
	if !strings.HasPrefix(target, "socket:[") || !strings.HasSuffix(target, "]") {
		return 0, false
	}
	inodeStr := target[len("socket:[") : len(target)-1]
	inode, err := strconv.ParseUint(inodeStr, 10, 64)
	if err != nil {
		return 0, false
	}
	return inode, true
}

