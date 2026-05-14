package userprofile

import (
	"strings"

	"github.com/a-digi/coco-iam/src/organizations/profile/entity"
	"github.com/a-digi/coco-server/server/media"
)

// effectiveLimits decides the mime allowlist and the byte cap the
// upload handler must enforce for a specific field + the detected
// mime. Pure — no I/O — so every branch is unit-tested independently
// from the HTTP handler.
//
// Rules:
//   - AcceptMime == "" → fall back to the media subsystem's default
//     allowlist (anything media.DetectAndValidateMime accepts).
//   - AcceptMime non-empty → comma-separated list of exact mime
//     matches; only those are allowed.
//   - MaxBytes == 0 → fall back to media.CapForMime(detectedMime).
//   - MaxBytes > 0 AND media.CapForMime(detectedMime) > 0 → use
//     the lower of the two so an admin can narrow, never widen,
//     the media defaults.
//   - MaxBytes > 0 AND media.CapForMime returns 0 (the detected
//     mime isn't in the media allowlist, which shouldn't happen
//     because the scanner rejects it first) → use MaxBytes verbatim.
//
// When the returned allowlist is empty the caller treats it as
// "media defaults"; when it's non-empty the caller must find the
// detected mime in it.
func effectiveLimits(f *entity.ProfileField, detectedMime string) (allow []string, cap int64) {
	if f != nil && strings.TrimSpace(f.AcceptMime) != "" {
		for _, part := range strings.Split(f.AcceptMime, ",") {
			if v := strings.TrimSpace(part); v != "" {
				allow = append(allow, v)
			}
		}
	}
	mediaCap := media.CapForMime(detectedMime)
	switch {
	case f == nil || f.MaxBytes <= 0:
		cap = mediaCap
	case mediaCap == 0:
		cap = int64(f.MaxBytes)
	case int64(f.MaxBytes) < mediaCap:
		cap = int64(f.MaxBytes)
	default:
		cap = mediaCap
	}
	return allow, cap
}

// mimeAllowed reports whether detectedMime is acceptable given a
// per-field allowlist. Empty allowlist means "media defaults apply,
// everything the scanner already accepted is fine".
func mimeAllowed(detectedMime string, allow []string) bool {
	if len(allow) == 0 {
		return true
	}
	for _, m := range allow {
		if m == detectedMime {
			return true
		}
	}
	return false
}
