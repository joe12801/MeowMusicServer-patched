# MeowMusicServer-patched Windows 部署教程

这份教程面向 **Windows 本机部署**，目标是把当前这版 `MeowMusicServer-patched` 跑起来，并支持：

- 本地音乐上传/播放
- 旧源优先
- YouTube 补源
- YouTube Cookie 自动同步

---

## 一、准备环境

建议系统：
- Windows 10 / 11

需要安装的软件：
1. **Git**
2. **Go**（建议 1.22+）
3. **Python 3.10+**
4. **FFmpeg**
5. **Node.js 22+**
6. **yt-dlp**

---

## 二、安装依赖

### 1. 安装 Git
下载并安装：
- https://git-scm.com/download/win

### 2. 安装 Go
下载并安装：
- https://go.dev/dl/

安装完成后，PowerShell 测试：

```powershell
go version
```

### 3. 安装 Python
下载并安装：
- https://www.python.org/downloads/windows/

安装时勾选：
- `Add Python to PATH`

测试：

```powershell
python --version
```

### 4. 安装 FFmpeg
可用任意 Windows 发行版，只要能在命令行调用：

```powershell
ffmpeg -version
ffprobe -version
```

如果没有 PATH，可以手动加到系统环境变量。

### 5. 安装 Node.js
下载并安装：
- https://nodejs.org/

建议：**Node.js 22 LTS**

测试：

```powershell
node -v
npm -v
```

### 6. 安装 yt-dlp 及 EJS 相关依赖

```powershell
python -m pip install -U yt-dlp[default]
```

测试：

```powershell
yt-dlp --version
```

---

## 三、获取代码

### 方式 1：直接下载 ZIP
把代码包解压到例如：

```text
C:\MeowMusicServer-patched
```

### 方式 2：Git clone

```powershell
git clone https://github.com/joe12801/MeowMusicServer-patched.git C:\MeowMusicServer-patched
```

---

## 四、启动项目

进入目录：

```powershell
cd C:\MeowMusicServer-patched
```

启动：

```powershell
python -m pip install -U yt-dlp[default]
go mod tidy
go build -o meowmusicserver.exe .
.\meowmusicserver.exe
```

默认端口通常是：

```text
2233
```

打开浏览器：

```text
http://127.0.0.1:2233/
```

---

## 五、YouTube Cookie 同步（Windows 本机）

如果你要启用 YouTube 补源，建议配好浏览器 Cookie 自动同步。

### 需要文件
当前方案已经有：
- `youtube_cookie_sync.py`
- `sync_cookie.bat`
- `open_youtube.bat`

### 同步思路
1. 本机 Chrome 保持登录 YouTube
2. 运行 `sync_cookie.bat`
3. 自动提取 Chrome Cookie
4. 自动上传到服务器 `/api/admin/youtube-cookie/update`

### 手动运行
双击：

```text
sync_cookie.bat
```

或者 PowerShell：

```powershell
python youtube_cookie_sync.py --browser chrome --profile Default --server http://127.0.0.1:2233 --token <你的token>
```

> 如果你不是部署在本机，而是部署到远端 Linux 服务器，请把 `--server` 改成远端地址。

---

## 六、功能说明

### 1. 音乐优先级
当前逻辑：

1. 缓存优先
2. 本地音乐其次
3. 旧源优先于 YouTube
4. 旧源失败时再走 YouTube 补源

### 2. 本地音乐
上传到：

```text
files/music/
```

Web UI 可：
- 上传
- 播放
- 重命名
- 删除
- 加入歌单

### 3. 缓存音乐
缓存目录：

```text
files/cache/music/
```

Web UI 可：
- 播放
- 删除缓存
- 转入本地
- 自动缓存开关

---

## 七、常见问题

### 1. 登录后刷新又掉线
检查：
- 前端是否强刷过缓存（Ctrl + F5）
- `files/users.json` 是否包含 `sessions`

### 2. YouTube 下载失败
检查：
- Chrome 是否登录 YouTube
- 是否已同步最新 cookie
- `yt-dlp[default]` 是否已安装
- `node -v` 是否 >= 20
- `ffmpeg / ffprobe` 是否可用

### 3. 本地音乐上传成功但不显示
检查：
- 是否进入“本地音乐”标签页
- 是否强刷页面缓存
- `GET /api/local-music` 是否能返回数据

### 4. 加入歌单没反应
检查：
- 是否已登录
- 是否已经有歌单
- 前端是否已强刷缓存

---

## 八、建议的 Windows 目录结构

```text
C:\MeowMusicServer-patched
├── meowmusicserver.exe
├── start.bat
├── files\
│   ├── music\
│   ├── cache\music\
│   ├── users.json
│   └── user_playlists.json
├── theme\
├── youtube_cookie_sync.py
├── sync_cookie.bat
└── open_youtube.bat
```

---

## 九、后续建议

如果你长期在 Windows 上跑，建议再补：
- 开机自启动
- 计划任务自动同步 YouTube cookie
- 定期备份 `files/music` 和 `files/users.json`

---

## 十、最短部署路径

如果你只想快速跑起来：

```powershell
cd C:\MeowMusicServer-patched
python -m pip install -U yt-dlp[default]
go mod tidy
go build -o meowmusicserver.exe .
.\meowmusicserver.exe
```

然后浏览器打开：

```text
http://127.0.0.1:2233/
```

如果要 YouTube 补源，再执行：

```text
sync_cookie.bat
```
