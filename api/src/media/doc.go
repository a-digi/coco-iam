// Package media hosts reusable image-transformation primitives
// any caller in the codebase can import. It is deliberately
// orthogonal to the vendored `coco-server/server/media` package
// (which owns MIME scanning + DB-backed file storage) — this
// package handles the bytes-in / bytes-out image processing
// layer that sits either side of it.
//
// The transform subpackage
// (github.com/a-digi/coco-iam/src/media/transform) provides:
//
//   - Decode / Encode for PNG, JPEG, GIF (decode only: WebP).
//   - Geometry — Resize (Contain / Cover / Stretch) + Crop /
//     CropCenter.
//   - Effects — Grayscale, Sepia, Invert, Brightness, Contrast,
//     Blur, Sharpen.
//   - Pipeline — fluent builder for chained operations.
//
// See plan/media-package/plan.md for the design rationale, the
// decisions locked up-front, and what's explicitly out of scope
// for this round (integration into avatar / profile-file upload
// handlers, on-demand URL-driven transforms, caching).
package media
