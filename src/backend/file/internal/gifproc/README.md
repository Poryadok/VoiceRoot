# GIF codec

`Convert(ctx, raw)` returns silent H.264/yuv420p MP4 with faststart and a static
WebP of the first composited frame. `Version` identifies the derivative recipe.
Odd edges are padded to even dimensions without rescaling; transparent pixels
are composited on black. GIF disposal previous/background and positive delays
are preserved. A zero delay becomes 10 centiseconds; duration metadata rounds
the normalized duration upward to whole seconds (minimum one).
Positive frame start timestamps retain centisecond precision. A final repeated
frame supplies the last hold and adds one 10 ms terminal packet; ffprobe checks
the complete MP4 duration against the source within 21 ms. H.264 B frames are
disabled to prevent reordering from shortening the final variable-duration hold.

Technical rejection budgets, before decompression: 200 MiB input, 4096 pixels
per edge, 16 Mi logical canvas pixels, 600 frames, 64 Mi cumulative logical
canvas pixels across frames, 120 seconds normalized duration. These are codec
safety budgets, not account admission limits. Each output must fit 5 MiB.
`errors.Is(err, ErrInvalid)` and `errors.Is(err, ErrTooLarge)` identify permanent
input/output rejections; process, filesystem and context errors remain separate.
All work has a 90 second deadline, subprocess output is bounded, and temporary
files are removed on every return. No input URL or network protocol is used.

Acceptance: real ffprobe verifies codec, pixel format, dimensions, duration and
absence of audio. Real decoded pixels verify animation and disposal/matte;
frame PTS verify variable timing. Negative tests cover malformed data, resource
limits, cancellation, dependency failure and cleanup.

Run `GIFPROC_REAL_CODEC=1 go test ./internal/gifproc -count=1 -v` in a Linux
environment with the File runtime's Alpine 3.20 `ffmpeg` package. The explicit
variable makes a missing ffmpeg/ffprobe fail; host runs without installed codec
skip only the real codec tests. Runtime qualification must execute without skips.
