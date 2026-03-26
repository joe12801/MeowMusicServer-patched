# MeowMusicServer Patched

A cleaned public patched version of MeowMusicServer for XiaoZhi / ESP32 devices.

[简体中文](./README_zh-CN.md) | English

---

## Project Overview

This repository is a patched public version of MeowMusicServer focused on **improving playback stability for XiaoZhi / ESP32 devices**.

Compared with the original behavior, this version prefers already cached and converted `music.mp3` files instead of using `stream_live` as the default playback source. This helps reduce common embedded-device playback issues such as:

- chunk parsing failures
- missing MP3 sync words
- playback stopping after a few seconds
- silent playback after a valid song match

---

## Main Changes

- Prefer stable cached `music.mp3` over `stream_live`
- Keep `stream_live` as a fallback/debug path instead of the default playback path
- Clean the repository for public release
- Preserve the original project structure while making playback behavior more device-friendly

---

## Recommended For

Use this patched version if your XiaoZhi / ESP32 device can:
- find songs correctly,
- receive valid audio links,
- but playback becomes unstable, stops after a few seconds, or fails during stream decoding.

---

## Quick Start

```bash
chmod +x start.sh
./start.sh
```

Then open:

```text
http://localhost:2233/app
```

---

## Recommended Validation

After startup, test:

```bash
curl "http://127.0.0.1:2233/stream_pcm?song=晴天&singer=周杰伦"
```

Check that returned `audio_url` / `audio_full_url` prefer:

```text
/cache/music/<artist>-<song>/music.mp3
```

instead of:

```text
/stream_live?... 
```

---

## Repository Notes

This public repository has been cleaned for release:
- no `.env`
- no runtime cache
- no build artifacts
- no intentional private device/session secrets included

---

## Documentation

- [README_zh-CN.md](./README_zh-CN.md)
- [快速开始](./快速开始.md)
- [本地部署指南](./本地部署指南.md)
- [USER_SYSTEM_README.md](./USER_SYSTEM_README.md)

---

## License

Follow the upstream project license unless otherwise stated.
