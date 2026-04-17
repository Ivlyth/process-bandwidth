package collector

import (
	"os"
	"testing"
)

func TestParseHexAddrIPv4(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"0100007F:0035", "127.0.0.1:53"},
		{"00000000:0000", "0.0.0.0:0"},
		{"0F02000A:1F90", "10.0.2.15:8080"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := parseHexAddr(tc.input, false)
			if got != tc.want {
				t.Errorf("parseHexAddr(%q, false): want %q, got %q", tc.input, tc.want, got)
			}
		})
	}
}

func TestParseHexAddrIPv6(t *testing.T) {
	// ::1 in /proc/net/tcp6 is "00000000000000000000000001000000" (4 LE groups)
	// Encoded as 4 groups of 4 bytes reversed: 00000000 00000000 00000000 01000000
	got := parseHexAddr("00000000000000000000000001000000:0050", true)
	if got != "[::1]:80" {
		t.Errorf("parseHexAddrIPv6: want [::1]:80, got %s", got)
	}
}

func TestNetResolverParseTCPFixture(t *testing.T) {
	// Write a fixture /proc/net/tcp file
	fixture := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:0035 00000000:0000 0A 00000000:00000000 00:00000000 00000000   101        0 12345 1 0000000000000000 100 0 0 10 0
   1: 0F02000A:1F90 0101A8C0:C3F4 01 00000000:00000000 00:00000000 00000000     0        0 67890 1 0000000000000000 20 4 28 10 -1
`
	tmpf, err := os.CreateTemp("", "proc_net_tcp")
	if err != nil {
		t.Skip("cannot create temp file:", err)
	}
	defer os.Remove(tmpf.Name())
	tmpf.WriteString(fixture)
	tmpf.Close()

	r := NewNetResolver(0)
	r.parseTCP(tmpf.Name(), "tcp4", false)

	// Check inode 12345 – local 127.0.0.1:53, remote 0.0.0.0:0
	_, ok := r.cache.Load(uint64(12345))
	if !ok {
		t.Fatal("inode 12345 not found in cache")
	}
	from := r.LookupByInode(12345)
	if from == nil {
		t.Fatal("LookupByInode(12345) returned nil")
	}
	if from.Protocol != "tcp4" {
		t.Errorf("Protocol: want tcp4, got %s", from.Protocol)
	}
	if from.Local != "127.0.0.1:53" {
		t.Errorf("Local: want 127.0.0.1:53, got %s", from.Local)
	}

	// inode 67890 – established connection
	from2 := r.LookupByInode(67890)
	if from2 == nil {
		t.Fatal("LookupByInode(67890) returned nil")
	}
	if from2.Local != "10.0.2.15:8080" {
		t.Errorf("Local: want 10.0.2.15:8080, got %s", from2.Local)
	}
}

func TestUitoa(t *testing.T) {
	cases := []struct {
		n    uint32
		want string
	}{
		{0, "0"},
		{1, "1"},
		{12345, "12345"},
		{4294967295, "4294967295"},
	}
	for _, tc := range cases {
		got := uitoa(tc.n)
		if got != tc.want {
			t.Errorf("uitoa(%d): want %q, got %q", tc.n, tc.want, got)
		}
	}
}
