package scanwatch

import (
	"net"
	"strconv"
	"strings"
)

// DefaultLogPrefix is the --log-prefix value the documented iptables
// setup rule uses (see plan/port-scan-detection/plan.md Phase B and
// docs/setup/port-scan-detection.md) — defined once here so the
// wiring that calls ParseLine and the operator-facing setup doc can
// never drift apart.
const DefaultLogPrefix = "coco-portscan: "

// Hit is one successfully parsed port-probe log line — a single
// packet that reached the end of the host's INPUT chain unmatched by
// any earlier accept rule, which is exactly the signature the
// documented iptables LOG rule (see
// plan/port-scan-detection/plan.md Phase B) captures.
type Hit struct {
	IP    string
	Port  int
	Proto string
}

// ParseLine extracts SRC=/DPT=/PROTO= fields from a standard iptables
// LOG line. Gated on logPrefix (the exact --log-prefix configured in
// the documented iptables rule) so this can't misfire on unrelated
// kernel log lines that happen to contain SRC=/DPT= for other
// reasons — an empty logPrefix disables that gate (matches
// everything), which callers should only pass in tests. Returns
// ok=false if the line doesn't match the prefix or is missing a
// required field.
func ParseLine(line, logPrefix string) (Hit, bool) {
	if logPrefix != "" && !strings.Contains(line, logPrefix) {
		return Hit{}, false
	}

	src, ok := extractField(line, "SRC=")
	if !ok || net.ParseIP(src) == nil {
		return Hit{}, false
	}
	dpt, ok := extractField(line, "DPT=")
	if !ok {
		return Hit{}, false
	}
	port, err := strconv.Atoi(dpt)
	if err != nil || port < 0 || port > 65535 {
		return Hit{}, false
	}
	proto, _ := extractField(line, "PROTO=")

	return Hit{IP: src, Port: port, Proto: proto}, true
}

// extractField returns the value of a "KEY=value" token in line —
// iptables' LOG format is space-separated "KEY=value" pairs, so the
// value ends at the next space or end of string.
func extractField(line, key string) (string, bool) {
	idx := strings.Index(line, key)
	if idx < 0 {
		return "", false
	}
	rest := line[idx+len(key):]
	if end := strings.IndexByte(rest, ' '); end >= 0 {
		return rest[:end], true
	}
	return rest, true
}
