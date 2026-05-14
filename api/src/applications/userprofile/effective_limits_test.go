package userprofile

import (
	"testing"

	"github.com/a-digi/coco-iam/src/organizations/profile/entity"
	"github.com/a-digi/coco-server/server/media"
)

func TestEffectiveLimits_PerFieldAcceptMimeUsedVerbatim(t *testing.T) {
	f := &entity.ProfileField{AcceptMime: "image/png, image/jpeg"}
	allow, _ := effectiveLimits(f, "image/png")
	if len(allow) != 2 || allow[0] != "image/png" || allow[1] != "image/jpeg" {
		t.Fatalf("allow list not split/trimmed correctly: %v", allow)
	}
}

func TestEffectiveLimits_EmptyAcceptMimeFallsBackToMediaDefaults(t *testing.T) {
	f := &entity.ProfileField{AcceptMime: ""}
	allow, _ := effectiveLimits(f, "image/png")
	if len(allow) != 0 {
		t.Fatalf("empty accept_mime should yield empty allow list, got %v", allow)
	}
	// And the handler's mimeAllowed contract says empty == accept
	// whatever the scanner already validated.
	if !mimeAllowed("image/png", allow) {
		t.Fatalf("empty allow should accept any detected mime")
	}
}

func TestEffectiveLimits_MaxBytesZeroFallsBackToMediaCap(t *testing.T) {
	f := &entity.ProfileField{MaxBytes: 0}
	_, cap := effectiveLimits(f, "image/png")
	if cap != media.CapForMime("image/png") {
		t.Fatalf("zero MaxBytes should use media default cap, got %d want %d",
			cap, media.CapForMime("image/png"))
	}
}

func TestEffectiveLimits_MaxBytesNarrowsButNeverWidens(t *testing.T) {
	// 1 MB cap on a field whose detected mime is PNG — media's
	// default cap for images is 5 MB, so per-field must win.
	f := &entity.ProfileField{MaxBytes: 1024 * 1024}
	_, cap := effectiveLimits(f, "image/png")
	if cap != 1024*1024 {
		t.Fatalf("want 1MB (per-field override), got %d", cap)
	}

	// A field that asks for MORE than media's default must still
	// be clamped to the media default — admins narrow, never widen.
	f2 := &entity.ProfileField{MaxBytes: 999 * 1024 * 1024}
	_, cap2 := effectiveLimits(f2, "image/png")
	if cap2 != media.CapForMime("image/png") {
		t.Fatalf("want media default cap (clamp), got %d", cap2)
	}
}
