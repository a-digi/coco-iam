// Package transform is the first-party image-transformation layer
// used across the codebase. It wraps github.com/disintegration/
// imaging for the heavy lifting (resize filters, geometric ops,
// most effects) and adds a fluent Pipeline plus a project-local
// Format / EncodeOpts API so callers speak one vocabulary.
//
// Golden rules for contributors:
//
//   - Every operation returns a FRESH image.Image. Never mutate
//     the input.
//   - Tests live next to the code and must write their output
//     files to testdata/out/ so a human can eyeball the gallery
//     after `go test ./src/media/transform/...`.
//   - No CGo dependencies. WebP decode via golang.org/x/image/
//     webp is OK; WebP encode is out of scope (requires libwebp).
//   - Animated GIFs are treated as "first frame only" — see
//     Decode's docs.
package transform
