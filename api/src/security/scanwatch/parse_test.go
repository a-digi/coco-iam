package scanwatch

import "testing"

const testPrefix = "coco-portscan: "

func TestParseLine(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		logPrefix string
		wantOK    bool
		want      Hit
	}{
		{
			name: "real journald -o cat line, TCP SYN to a closed port",
			line: "coco-portscan: IN=eth0 OUT= MAC=ff:ff:ff:ff:ff:ff:00:00:00:00:00:00:08:00 SRC=203.0.113.7 DST=10.0.0.5 LEN=40 TOS=0x00 PREC=0x00 TTL=64 ID=12345 PROTO=TCP SPT=54321 DPT=22 WINDOW=1024 RES=0x00 SYN URGP=0",
			logPrefix: testPrefix,
			wantOK:    true,
			want:      Hit{IP: "203.0.113.7", Port: 22, Proto: "TCP"},
		},
		{
			name:      "plain syslog file line with a syslog header prefix",
			line:      "Jul 26 10:00:00 host kernel: coco-portscan: IN=eth0 OUT= SRC=198.51.100.9 DST=10.0.0.5 PROTO=UDP SPT=40000 DPT=3306 LEN=60",
			logPrefix: testPrefix,
			wantOK:    true,
			want:      Hit{IP: "198.51.100.9", Port: 3306, Proto: "UDP"},
		},
		{
			name:      "DPT is the last field with nothing following",
			line:      "coco-portscan: SRC=203.0.113.7 PROTO=TCP DPT=8080",
			logPrefix: testPrefix,
			wantOK:    true,
			want:      Hit{IP: "203.0.113.7", Port: 8080, Proto: "TCP"},
		},
		{
			name:      "IPv6 source",
			line:      "coco-portscan: SRC=2001:db8::1 DST=fe80::1 PROTO=TCP DPT=443",
			logPrefix: testPrefix,
			wantOK:    true,
			want:      Hit{IP: "2001:db8::1", Port: 443, Proto: "TCP"},
		},
		{
			name:      "unrelated kernel log line must not misfire",
			line:      "Jul 26 10:00:00 host kernel: usb 1-1: new high-speed USB device number 2 using xhci_hcd",
			logPrefix: testPrefix,
			wantOK:    false,
		},
		{
			name:      "matches the prefix but missing SRC entirely",
			line:      "coco-portscan: DST=10.0.0.5 PROTO=TCP DPT=22",
			logPrefix: testPrefix,
			wantOK:    false,
		},
		{
			name:      "matches the prefix but missing DPT entirely",
			line:      "coco-portscan: SRC=203.0.113.7 PROTO=TCP",
			logPrefix: testPrefix,
			wantOK:    false,
		},
		{
			name:      "malformed SRC value is not a valid IP",
			line:      "coco-portscan: SRC=not-an-ip PROTO=TCP DPT=22",
			logPrefix: testPrefix,
			wantOK:    false,
		},
		{
			name:      "malformed DPT value is not numeric",
			line:      "coco-portscan: SRC=203.0.113.7 PROTO=TCP DPT=not-a-port",
			logPrefix: testPrefix,
			wantOK:    false,
		},
		{
			name:      "DPT out of valid port range",
			line:      "coco-portscan: SRC=203.0.113.7 PROTO=TCP DPT=99999",
			logPrefix: testPrefix,
			wantOK:    false,
		},
		{
			name:      "right-shaped line but wrong prefix must not match",
			line:      "some-other-rule: SRC=203.0.113.7 PROTO=TCP DPT=22",
			logPrefix: testPrefix,
			wantOK:    false,
		},
		{
			name:      "empty logPrefix disables the gate entirely",
			line:      "no prefix at all SRC=203.0.113.7 PROTO=TCP DPT=22",
			logPrefix: "",
			wantOK:    true,
			want:      Hit{IP: "203.0.113.7", Port: 22, Proto: "TCP"},
		},
		{
			name:      "missing PROTO is tolerated — proto is optional",
			line:      "coco-portscan: SRC=203.0.113.7 DPT=22",
			logPrefix: testPrefix,
			wantOK:    true,
			want:      Hit{IP: "203.0.113.7", Port: 22, Proto: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseLine(tt.line, tt.logPrefix)
			if ok != tt.wantOK {
				t.Fatalf("ParseLine() ok = %v, want %v (line=%q)", ok, tt.wantOK, tt.line)
			}
			if !ok {
				return
			}
			if got != tt.want {
				t.Fatalf("ParseLine() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
