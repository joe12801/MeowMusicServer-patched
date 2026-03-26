# MeowMusicServer Patched

A cleaned public patched version of MeowMusicServer for XiaoZhi / embedded devices.

## What changed in this patched version
This fork focuses on **playback stability for embedded devices**.

### Main changes
- Prefer **stable cached `music.mp3`** over `stream_live` as the default playback target
- Reduce the chance of chunk parsing issues on embedded devices
- Keep `stream_live` as a fallback / debug path instead of the primary playback path
- Remove sensitive local deployment artifacts before publishing

## Recommended use case
Use this patched version if your XiaoZhi / ESP32-based device:
- can search songs correctly,
- can get valid audio links,
- but playback becomes unstable, cuts off after a few seconds, or fails to decode streaming chunks.

## Current publishing notes
This repository is a **clean public version** prepared for release:
- no `.env`
- no runtime cache
- no build artifacts
- no real private device/session secrets intentionally included

## Upstream origin
Original project idea and structure are based on the upstream MeowMusicServer project.

## Quick start
```bash
chmod +x start.sh
./start.sh
```

Then open:
```text
http://localhost:2233/app
```

## Recommended validation
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

## Docs
- [README_zh-CN.md](./README_zh-CN.md)
- [快速开始](./快速开始.md)
- [本地部署指南](./本地部署指南.md)
- [USER_SYSTEM_README.md](./USER_SYSTEM_README.md)

## License
Follow the upstream project license unless otherwise stated.
