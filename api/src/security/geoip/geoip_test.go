package geoip

import "testing"

func TestEncodeIP_NilReturnsNotOK(t *testing.T) {
	if _, _, ok := EncodeIP(nil); ok {
		t.Fatal("EncodeIP(nil) ok = true, want false")
	}
}

func TestIsLoopbackOrPrivate(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"10.0.0.1", true},
		{"172.16.5.9", true},
		{"192.168.1.1", true},
		{"203.0.113.7", false},
		{"91.230.168.240", false},
		{"not-an-ip", false},
	}
	for _, c := range cases {
		if got := IsLoopbackOrPrivate(c.ip); got != c.want {
			t.Errorf("IsLoopbackOrPrivate(%q) = %v, want %v", c.ip, got, c.want)
		}
	}
}

func TestNoopLookup_AlwaysReturnsFalseAndDisabled(t *testing.T) {
	var l Lookup = NoopLookup{}

	if l.Enabled() {
		t.Fatal("NoopLookup.Enabled() = true, want false")
	}

	info, ok := l.Lookup("203.0.113.7")
	if ok {
		t.Fatalf("NoopLookup.Lookup() ok = true, want false")
	}
	if info != (Info{}) {
		t.Fatalf("NoopLookup.Lookup() info = %+v, want zero value", info)
	}
}
