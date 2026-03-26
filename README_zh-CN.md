# MeowMusicServer Patched

这是一个面向 **小智 / ESP32 嵌入式设备** 的 MeowMusicServer 清理发布版与稳定性补丁版。

## 这个补丁版改了什么
这个版本重点解决的是：

> **嵌入式设备播放在线音乐时，优先使用稳定缓存后的 `music.mp3`，而不是优先使用 `stream_live` 实时流。**

### 主要改动
- 默认优先返回稳定缓存后的 `music.mp3`
- 降低嵌入式设备端出现 chunk 解析失败、MP3 同步字错误、播放几秒后断流等问题的概率
- 保留 `stream_live` 作为调试/备用路径，而不是默认播放入口
- 清理了发布时不应包含的本地运行痕迹

## 适合谁用
如果你遇到以下问题，这个版本更适合你：
- 小智能查到歌，但播放几秒就断
- 设备端日志出现 chunk 解析失败
- 设备端日志出现 MP3 sync word 找不到
- 服务端已经能找到歌曲和音频链接，但设备端播放不稳

## 当前发布说明
这是一个 **clean public 版**，已去除或尽量避免：
- `.env`
- 运行缓存
- 编译产物
- 敏感本地部署文件
- 明确的私有设备/会话凭据

## 与原版的关系
此项目基于上游 MeowMusicServer 的结构和思路整理而来，当前版本主要强调：
- 更适合嵌入式设备播放
- 更适合作为公开分享的 clean 版本

## 快速启动
```bash
chmod +x start.sh
./start.sh
```

然后访问：
```text
http://localhost:2233/app
```

## 推荐验证方法
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

## 文档
- [README.md](./README.md)
- [快速开始](./快速开始.md)
- [本地部署指南](./本地部署指南.md)
- [USER_SYSTEM_README.md](./USER_SYSTEM_README.md)

## License
默认遵循上游项目许可证，除非仓库中另有说明。
