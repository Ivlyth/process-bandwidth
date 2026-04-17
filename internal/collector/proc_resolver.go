package collector

import (
	"os"
	"strings"
	"sync"

	"github.com/Ivlyth/process-bandwidth/internal/model"
)

// ProcResolver asynchronously resolves process metadata (name, cmdline)
// from /proc/PID/comm and /proc/PID/cmdline.
//
// Each PID is resolved exactly once (sync.Once semantics). If the process
// exits before we resolve it, the fields remain at their zero values.
type ProcResolver struct {
	resolving sync.Map // map[uint32]struct{} – tracks in-flight / completed
}

// EnsureResolved schedules a background goroutine to resolve name/cmdline
// for proc if it hasn't been resolved yet. Subsequent calls for the same PID
// are no-ops.
func (r *ProcResolver) EnsureResolved(pid uint32, proc *model.Process) {
	if _, loaded := r.resolving.LoadOrStore(pid, struct{}{}); loaded {
		return
	}
	go r.resolve(pid, proc)
}

func (r *ProcResolver) resolve(pid uint32, proc *model.Process) {
	name := readComm(pid)
	cmdline := readCmdline(pid)
	proc.SetMeta(name, cmdline)
}

// readComm reads /proc/PID/comm which contains the process name (max 15 chars).
func readComm(pid uint32) string {
	b, err := os.ReadFile(pidPath(pid, "comm"))
	if err != nil {
		return "unknown"
	}
	return strings.TrimRight(string(b), "\n\x00")
}

// readCmdline reads /proc/PID/cmdline and converts NUL-separated args to a
// space-separated string.
func readCmdline(pid uint32) string {
	b, err := os.ReadFile(pidPath(pid, "cmdline"))
	if err != nil || len(b) == 0 {
		return ""
	}
	// cmdline has args separated by NUL bytes; replace with spaces.
	return strings.TrimRight(strings.ReplaceAll(string(b), "\x00", " "), " ")
}

func pidPath(pid uint32, file string) string {
	// Avoid fmt.Sprintf in the hot path – hand-build the string.
	return "/proc/" + uitoa(pid) + "/" + file
}

// uitoa converts a uint32 to its decimal string representation.
func uitoa(n uint32) string {
	if n == 0 {
		return "0"
	}
	buf := [10]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
