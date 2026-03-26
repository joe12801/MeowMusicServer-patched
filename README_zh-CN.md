# MeowMusicServer Patched

这是一个面向 **小智 / ESP32 设备** 的 MeowMusicServer 公开补丁版。

English | [简体中文](./README_zh-CN.md)

---

## 项目简介

这个仓库是一个针对 **小智 / ESP32 设备音乐播放稳定性** 做过修正的公开版本。

与原始行为相比，这个版本优先返回已经缓存并转码完成的 `music.mp3` 文件，而不是默认优先返回 `stream_live` 实时流。这样可以降低嵌入式设备端常见的播放问题，例如：

- chunk 解析失败
- MP3 同步字错误
- 播放几秒后断流
- 找到歌曲但没有稳定声音输出

---

## 主要改动

- 默认优先返回稳定缓存后的 `music.mp3`
- `stream_live` 保留为调试/备用路径，而不是默认播放入口
- 清理仓库以便公开发布
- 在尽量保留原项目结构的前提下，让它更适合嵌入式设备播放

---

## 适合谁使用

如果你的小智 / ESP32 设备已经能够：
- 正常搜到歌曲
- 拿到正确的音频链接

但依然出现：
- 播放几秒就断
- 实时流解码失败
- 无声播放

那么这个补丁版更适合你。

---

## 快速开始

```bash
chmod +x start.sh
./start.sh
```

然后访问：

```text
http://localhost:2233/app
```

---

## 推荐验证方式

启动后建议测试：

```bash
curl "http://127.0.0.1:2233/stream_pcm?song=晴天&singer=周杰伦"
```

重点看返回的：
- `audio_url`
- `audio_full_url`

是否优先指向：

```text
/cache/music/<歌手>-<歌名>/music.mp3
```

而不是：

```text
/stream_live?... 
```

---

## 仓库说明

这个公开仓库已经做过清理：
- 不包含 `.env`
- 不包含运行缓存
- 不包含编译产物
- 不故意包含真实设备/会话敏感信息

---

## 文档

- [README.md](./README.md)
- [快速开始](./快速开始.md)
- [本地部署指南](./本地部署指南.md)
- [USER_SYSTEM_README.md](./USER_SYSTEM_README.md)

---

## License

默认遵循上游项目许可证，除非仓库中另有说明。
