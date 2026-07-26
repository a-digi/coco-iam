//go:build !linux && !darwin && !windows

package firewall

import "github.com/a-digi/coco-logger/logger"

// detectPlatform for every OS besides Linux, macOS, and Windows —
// e.g. FreeBSD. No backend is planned for these; enforcement stays at
// the application layer only.
func detectPlatform(log logger.Logger) Banner {
	return NewNoopBanner("OS-level firewall enforcement not implemented for this platform")
}
