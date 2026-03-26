# Release Notes - v1.0.0

## MeowMusicServer Patched

This release focuses on improving music playback stability for XiaoZhi / ESP32 devices.

### Highlights
- Prefer cached `music.mp3` over `stream_live` by default
- Reduce playback instability caused by real-time chunked streaming
- Keep `stream_live` available as a fallback/debug path
- Publish a cleaned version suitable for public sharing

### Recommended For
Use this release if your device:
- can search songs correctly,
- receives valid music links,
- but playback becomes unstable, stops after a few seconds, or fails while decoding live streams.

### Notes
- This release focuses on server-side playback stability improvements.
- Device-side audio pipeline issues may still require firmware-side fixes.
- Keep your own runtime `.env`, secrets, and caches outside the public repository.
