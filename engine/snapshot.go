package engine

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"sync"
	"time"
)

func auto(s uint64, t time.Duration, useBit ...bool) (float64, string) {
	baseF := float64(1024)

	u1 := " "
	u2 := "B"

	rate := float64(s) / (float64(t) / float64(time.Second))
	if len(useBit) > 0 && useBit[0] {
		u2 = "b"
		rate = rate * 8.0
	}

	if rate < baseF { // less than 1KB

	} else if rate < baseF*baseF { // less than 1MB
		u1 = "K"
		rate = rate / baseF
	} else if rate < baseF*baseF*baseF { // less than 1GB
		u1 = "M"
		rate = rate / baseF / baseF
	} else { // G
		u1 = "G"
		rate = rate / baseF / baseF / baseF
	}

	return rate, fmt.Sprintf("%s%sps", u1, u2)
}

var SnapshotLock sync.Mutex

type RWSnapshot struct {
	OutgoingBytes uint64
	IncomingBytes uint64
	At            time.Time
	Duration      time.Duration
}

func (s *RWSnapshot) MarshalJSON() ([]byte, error) {
	buf := &bytes.Buffer{}
	buf.WriteByte('{')
	buf.WriteString(fmt.Sprintf(`"outgoingBytes":%d,`, s.OutgoingBytes))
	buf.WriteString(fmt.Sprintf(`"incomingBytes":%d,`, s.IncomingBytes))
	buf.WriteString(fmt.Sprintf(`"at":"%s",`, s.At.Format("2006-01-02 15:04:05.000")))
	buf.WriteString(fmt.Sprintf(`"incoming":"%s",`, s.IncomingRateAutobS()))
	buf.WriteString(fmt.Sprintf(`"outgoing":"%s"`, s.OutgoingRateAutobS()))
	buf.WriteByte('}')
	return ioutil.ReadAll(buf)
}

func (s *RWSnapshot) Clone() RWSnapshot {
	return RWSnapshot{
		OutgoingBytes: s.OutgoingBytes,
		IncomingBytes: s.IncomingBytes,
		At:            s.At,
		Duration:      s.Duration,
	}
}

func (s *RWSnapshot) IncomingRateBps() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.IncomingBytes) / (float64(s.Duration) / float64(time.Second))
}

func (s *RWSnapshot) IncomingRateKBps() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.IncomingBytes) / (float64(s.Duration) / float64(time.Second)) / 1024.0
}

func (s *RWSnapshot) IncomingRateMBps() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.IncomingBytes) / (float64(s.Duration) / float64(time.Second)) / 1024.0 / 1024.0
}

func (s *RWSnapshot) IncomingRateGBps() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.IncomingBytes) / (float64(s.Duration) / float64(time.Second)) / 1024.0 / 1024.0 / 1024.0
}

func (s *RWSnapshot) IncomingRatebps() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.IncomingBytes) / (float64(s.Duration) / float64(time.Second)) * 8.0
}

func (s *RWSnapshot) IncomingRateKbps() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.IncomingBytes) / (float64(s.Duration) / float64(time.Second)) / 1024.0 * 8.0
}

func (s *RWSnapshot) IncomingRateMbps() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.IncomingBytes) / (float64(s.Duration) / float64(time.Second)) / 1024.0 / 1024.0 * 8.0
}

func (s *RWSnapshot) IncomingRateGbps() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.IncomingBytes) / (float64(s.Duration) / float64(time.Second)) / 1024.0 / 1024.0 / 1024.0 * 8.0
}

func (s *RWSnapshot) IncomingRateAutoB() (float64, string) {
	if s.Duration == 0 {
		return 0, " Bps"
	}
	return auto(s.IncomingBytes, s.Duration)
}

func (s *RWSnapshot) IncomingRateAutob() (float64, string) {
	if s.Duration == 0 {
		return 0, " bps"
	}
	return auto(s.IncomingBytes, s.Duration, true)
}
func (s *RWSnapshot) IncomingRateAutoBS() string {
	rate, unit := s.IncomingRateAutoB()
	return fmt.Sprintf("%7.2f %s", rate, unit)
}

func (s *RWSnapshot) IncomingRateAutobS() string {
	rate, unit := s.IncomingRateAutob()
	return fmt.Sprintf("%7.2f %s", rate, unit)
}

// -----------------

func (s *RWSnapshot) OutgoingRateBps() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.OutgoingBytes) / (float64(s.Duration) / float64(time.Second))
}

func (s *RWSnapshot) OutgoingRateKBps() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.OutgoingBytes) / (float64(s.Duration) / float64(time.Second)) / 1024.0
}

func (s *RWSnapshot) OutgoingRateMBps() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.OutgoingBytes) / (float64(s.Duration) / float64(time.Second)) / 1024.0 / 1024.0
}

func (s *RWSnapshot) OutgoingRateGBps() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.OutgoingBytes) / (float64(s.Duration) / float64(time.Second)) / 1024.0 / 1024.0 / 1024.0
}

func (s *RWSnapshot) OutgoingRatebps() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.OutgoingBytes) / (float64(s.Duration) / float64(time.Second)) * 8.0
}

func (s *RWSnapshot) OutgoingRateKbps() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.OutgoingBytes) / (float64(s.Duration) / float64(time.Second)) / 1024.0 * 8.0
}

func (s *RWSnapshot) OutgoingRateMbps() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.OutgoingBytes) / (float64(s.Duration) / float64(time.Second)) / 1024.0 / 1024.0 * 8.0
}

func (s *RWSnapshot) OutgoingRateGbps() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.OutgoingBytes) / (float64(s.Duration) / float64(time.Second)) / 1024.0 / 1024.0 / 1024.0 * 8.0
}

func (s *RWSnapshot) OutgoingRateAutoB() (float64, string) {
	if s.Duration == 0 {
		return 0, " Bps"
	}
	return auto(s.OutgoingBytes, s.Duration)
}

func (s *RWSnapshot) OutgoingRateAutob() (float64, string) {
	if s.Duration == 0 {
		return 0, " bps"
	}
	return auto(s.OutgoingBytes, s.Duration, true)
}

func (s *RWSnapshot) OutgoingRateAutoBS() string {
	rate, unit := s.OutgoingRateAutoB()
	return fmt.Sprintf("%7.2f %s", rate, unit)
}

func (s *RWSnapshot) OutgoingRateAutobS() string {
	rate, unit := s.OutgoingRateAutob()
	return fmt.Sprintf("%7.2f %s", rate, unit)
}

// ------------

func (s *RWSnapshot) TotalRatebps() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.IncomingBytes+s.OutgoingBytes) / (float64(s.Duration) / float64(time.Second)) * 8.0
}

func (s *RWSnapshot) TotalRateKbps() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.IncomingBytes+s.OutgoingBytes) / (float64(s.Duration) / float64(time.Second)) / 1024.0 * 8.0
}

func (s *RWSnapshot) TotalRateMbps() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.IncomingBytes+s.OutgoingBytes) / (float64(s.Duration) / float64(time.Second)) / 1024.0 / 1024.0 * 8.0
}

func (s *RWSnapshot) TotalRateGbps() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.IncomingBytes+s.OutgoingBytes) / (float64(s.Duration) / float64(time.Second)) / 1024.0 / 1024.0 / 1024.0
}

func (s *RWSnapshot) TotalRateBps() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.IncomingBytes+s.OutgoingBytes) / (float64(s.Duration) / float64(time.Second))
}

func (s *RWSnapshot) TotalRateKBps() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.IncomingBytes+s.OutgoingBytes) / (float64(s.Duration) / float64(time.Second)) / 1024.0
}

func (s *RWSnapshot) TotalRateMBps() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.IncomingBytes+s.OutgoingBytes) / (float64(s.Duration) / float64(time.Second)) / 1024.0 / 1024.0
}

func (s *RWSnapshot) TotalRateGBps() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.IncomingBytes+s.OutgoingBytes) / (float64(s.Duration) / float64(time.Second)) / 1024.0 / 1024.0 / 1024.0
}

func (s *RWSnapshot) TotalRateAutoB() (float64, string) {
	if s.Duration == 0 {
		return 0, " Bps"
	}
	return auto(s.OutgoingBytes+s.IncomingBytes, s.Duration)
}

func (s *RWSnapshot) TotalRateAutob() (float64, string) {
	if s.Duration == 0 {
		return 0, " bps"
	}
	return auto(s.OutgoingBytes+s.IncomingBytes, s.Duration, true)
}

func (s *RWSnapshot) TotalRateAutoBS() string {
	rate, unit := s.TotalRateAutoB()
	return fmt.Sprintf("%7.2f %s", rate, unit)
}

func (s *RWSnapshot) TotalRateAutobS() string {
	rate, unit := s.TotalRateAutob()
	return fmt.Sprintf("%7.2f %s", rate, unit)
}
