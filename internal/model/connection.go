package model

import (
	"sync/atomic"
	"time"
	"unsafe"
)

// FDClass classifies a file descriptor.
type FDClass uint8

const (
	FDClassUnknown FDClass = 0
	FDClassSocket  FDClass = 1
	FDClassFile    FDClass = 2
	FDClassPipe    FDClass = 3
)

func (c FDClass) String() string {
	switch c {
	case FDClassSocket:
		return "socket"
	case FDClassFile:
		return "file"
	case FDClassPipe:
		return "pipe"
	default:
		return "unknown"
	}
}

// ConnectionInfo holds resolved network endpoint information for a socket FD.
// It is populated asynchronously by the NetResolver.
type ConnectionInfo struct {
	Inode    uint64
	Local    string // "ip:port" or "" for Unix sockets
	Remote   string // "ip:port" or path for Unix sockets
	Protocol string // "tcp4", "tcp6", "udp4", "udp6", "unix"
}

// Connection represents one open file descriptor within a process.
type Connection struct {
	FD    uint32
	Class FDClass

	// Socket fields – valid when Class == FDClassSocket.
	// Protected by atomic pointer swap (set once, never mutated).
	info unsafe.Pointer // *ConnectionInfo

	// File fields – valid when Class == FDClassFile or FDClassPipe.
	Path string

	IO *IOCounter

	createdAt    time.Time
	lastActivity atomic.Int64 // unix nano
}

// NewConnection creates a Connection for the given FD and class.
func NewConnection(fd uint32, class FDClass, histLen int) *Connection {
	c := &Connection{
		FD:        fd,
		Class:     class,
		IO:        NewIOCounter(histLen),
		createdAt: time.Now(),
	}
	c.lastActivity.Store(time.Now().UnixNano())
	return c
}

// SetInfo atomically stores resolved ConnectionInfo (called by NetResolver).
func (c *Connection) SetInfo(info *ConnectionInfo) {
	atomic.StorePointer(&c.info, unsafe.Pointer(info))
}

// Info returns the resolved ConnectionInfo, or nil if not yet resolved.
func (c *Connection) Info() *ConnectionInfo {
	p := atomic.LoadPointer(&c.info)
	if p == nil {
		return nil
	}
	return (*ConnectionInfo)(p)
}

// Touch updates the last-activity timestamp.
func (c *Connection) Touch() {
	c.lastActivity.Store(time.Now().UnixNano())
}

// LastActivity returns when the connection last had I/O activity.
func (c *Connection) LastActivity() time.Time {
	return time.Unix(0, c.lastActivity.Load())
}

// CreatedAt returns when the connection was first seen.
func (c *Connection) CreatedAt() time.Time { return c.createdAt }

// IsIdle returns true if the connection has had no activity for the given duration.
func (c *Connection) IsIdle(timeout time.Duration) bool {
	return time.Since(c.LastActivity()) > timeout
}
