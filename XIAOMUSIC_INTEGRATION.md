# XiaoMusic 补源接入说明

这个仓库已经接入了 `xiaomusic` 作为补源引擎。

## 已接入能力

- 搜索聚合：`/api/search?query=关键词`
  - 先查本地 sources / 本地缓存 / 原有 API
  - 再补查 xiaomusic
- 播放代理：`/api/xiaomusic/stream?payload=...`
  - 服务端向 xiaomusic 请求真实播放地址
  - 302 跳转到真实音频 URL
- 歌词代理：`/api/xiaomusic/lyric?payload=...`
  - 服务端向 xiaomusic 请求歌词

## 必需环境变量

### 1) XiaoMusic 服务地址

```bash
XIAOMUSIC_BASE_URL=http://127.0.0.1:8090
```

示例：

```bash
export XIAOMUSIC_BASE_URL=http://127.0.0.1:8090
```

### 2) 如果 xiaomusic 开了认证

```bash
XIAOMUSIC_AUTH='Basic xxxxxxxxxxxxx'
```

程序会把它原样放进 HTTP `Authorization` 请求头。

## 当前搜索顺序

`/api/search?query=xxx` 会按下面顺序聚合：

1. `sources.json`
2. 本地音乐
3. 原有外部 API
4. xiaomusic 在线搜索

并按 `title + artist` 去重，最多返回 20 条。

## 新增路由

- `GET /api/xiaomusic/stream?payload=...`
- `GET /api/xiaomusic/lyric?payload=...`

## 部署前自查

1. 先确认 xiaomusic 已正常运行
2. 本机执行：

```bash
curl 'http://127.0.0.1:8090/api/search/online?keyword=晴天&plugin=all&page=1&limit=5'
```

如果 xiaomusic 有鉴权，请带上对应 `Authorization` 头。

3. 再启动 MeowMusicServer：

```bash
export XIAOMUSIC_BASE_URL=http://127.0.0.1:8090
go run .
```

## 说明

如果 xiaomusic 没启动、地址不通、鉴权失败，主搜索接口仍会继续返回原有来源的结果，不会把整个搜索打挂。
