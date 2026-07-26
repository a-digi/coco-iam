package scanwatch

import "context"

// noopSource is the fallback when no log source is available — Start
// is a no-op and Lines never yields anything, so the Watcher simply
// never has data to aggregate. Mirrors firewall.NoopBanner's shape.
type noopSource struct {
	detail string
}

func newNoopSource(detail string) *noopSource {
	return &noopSource{detail: detail}
}

func (n *noopSource) Start(ctx context.Context) error { return nil }
func (n *noopSource) Lines() <-chan string            { return nil }
func (n *noopSource) Name() string                    { return "none" }
func (n *noopSource) Available() bool                 { return false }
func (n *noopSource) Detail() string                  { return n.detail }
