// Package scanwatch ingests OS-level firewall log lines to detect port
// scanning against the host — traffic to ports coco-iam isn't
// listening on, which is architecturally invisible to the app-layer
// rate limiter in ipguard. coco-iam never detects this itself; it
// only consumes a log line format the kernel/iptables already
// produce, via whichever source is actually available on this host.
// See plan/port-scan-detection/plan.md Phase B.
package scanwatch

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/a-digi/coco-logger/logger"
)

// DefaultSyslogFilePath is where the syslog-file fallback looks by
// default (Alpine's busybox syslogd target) — overridable by callers
// that pass a different path to Detect.
const DefaultSyslogFilePath = "/var/log/messages"

// lineChanBuffer bounds how many not-yet-consumed lines a Source holds
// before newer lines are dropped rather than blocking the underlying
// scan/tail loop — a burst of scan traffic must never back-pressure
// into the log reader itself.
const lineChanBuffer = 256

// Source streams raw kernel/firewall log lines. Detect picks exactly
// one implementation per process lifetime; the chosen Source's
// Start/Lines are used for as long as the process runs.
type Source interface {
	// Start begins streaming in a background goroutine until ctx is
	// cancelled, closing the Lines channel when it stops. Safe to call
	// at most once.
	Start(ctx context.Context) error
	Lines() <-chan string
	// Name identifies the source in use: "journald", "syslog_file", or
	// "none" if Available() is false.
	Name() string
	Available() bool
	// Detail explains why Available() is false. Empty when available.
	Detail() string
}

// Detect prefers journald (works on Ubuntu, and any other host that
// happens to have journalctl installed — detected by binary presence,
// not by GOOS, the same convention firewall.Detect already uses),
// falling back to tailing syslogFilePath (the default for Alpine,
// which has no journald). Never returns nil; a missing log source
// degrades to "scan detection unavailable" rather than failing
// startup — this feature is additive visibility, not something
// anything else depends on.
func Detect(log logger.Logger, syslogFilePath string) Source {
	return detect(log, syslogFilePath, exec.LookPath, os.Stat)
}

type lookPathFunc func(file string) (string, error)
type statFunc func(name string) (os.FileInfo, error)

func detect(log logger.Logger, syslogFilePath string, lookPath lookPathFunc, stat statFunc) Source {
	if s := detectJournald(log, lookPath); s != nil {
		return s
	}
	if syslogFilePath != "" {
		if _, err := stat(syslogFilePath); err == nil {
			return newSyslogFileSource(log, syslogFilePath)
		}
	}
	detail := fmt.Sprintf("neither journalctl nor a readable syslog file (%s) was found", syslogFilePath)
	if log != nil {
		log.Warning("scanwatch: no log source available (%s) — scan detection disabled", detail)
	}
	return newNoopSource(detail)
}
