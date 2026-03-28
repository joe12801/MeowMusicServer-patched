package main

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Source is an alias for MusicItem (used in sources.json)
type Source = MusicItem

// Download file from URL
func downloadFile(filepath string, url string) error {
	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

// Get IP address from request
func IPhandler(r *http.Request) (string, error) {
	ip := r.Header.Get("X-Real-IP")
	if ip == "" {
		ip = r.Header.Get("X-Forwarded-For")
	}
	if ip == "" {
		ip, _, _ = net.SplitHostPort(r.RemoteAddr)
	}
	return ip, nil
}

// Read sources from sources.json
func readSources() []Source {
	file, err := os.Open("sources.json")
	if err != nil {
		return []Source{}
	}
	defer file.Close()

	var sources []Source
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&sources)
	if err != nil {
		return []Source{}
	}
	return sources
}

// Read music from cache folder path like ./files/cache/music/Artist-Song or ./files/cache/music/<source>/Artist-Song
func readFromCache(path string) (MusicItem, bool) {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return MusicItem{}, false
	}
	rel, err := filepath.Rel(cacheMusicRoot, path)
	if err == nil {
		if item, ok := buildCacheMusicItem(filepath.ToSlash(rel)); ok {
			return item, true
		}
	}
	return buildCacheMusicItem(filepath.Base(path))
}

func buildCacheMusicItem(folder string) (MusicItem, bool) {
	folder = resolveCacheFolder(folder)
	if folder == "" {
		return MusicItem{}, false
	}

	dirPath := cacheDirPath(folder)
	mp3Path := filepath.Join(dirPath, "music.mp3")
	if info, err := os.Stat(mp3Path); err != nil || info.IsDir() || info.Size() < 1024 {
		return MusicItem{}, false
	}

	sourceType, artist, title, leaf := parseCacheFolder(folder)
	basePath := cacheBaseURL(folder)
	item := MusicItem{
		Title:        title,
		Artist:       artist,
		Filename:     leaf + ".mp3",
		AudioURL:     basePath + "/music.mp3",
		AudioFullURL: basePath + "/music.mp3",
		Duration:     GetDuration(mp3Path),
		FromCache:    true,
		SourceType:   sourceType,
	}
	if _, err := os.Stat(filepath.Join(dirPath, "music.m3u8")); err == nil {
		item.M3U8URL = basePath + "/music.m3u8"
	}
	if _, err := os.Stat(filepath.Join(dirPath, "lyric.lrc")); err == nil {
		item.LyricURL = basePath + "/lyric.lrc"
	}
	coverCandidates := []string{"cover.jpg", "cover.jpeg", "cover.png", "cover.webp"}
	for _, cover := range coverCandidates {
		if _, err := os.Stat(filepath.Join(dirPath, cover)); err == nil {
			item.CoverURL = basePath + "/" + cover
			break
		}
	}
	return item, true
}

func findExactCacheMusic(song, singer string) (MusicItem, bool) {
	wantSong := toLower(strings.TrimSpace(song))
	wantSinger := toLower(strings.TrimSpace(singer))
	for _, folder := range listCacheFolders() {
		item, ok := buildCacheMusicItem(folder)
		if !ok {
			continue
		}
		if toLower(strings.TrimSpace(item.Title)) != wantSong {
			continue
		}
		if wantSinger != "" && toLower(strings.TrimSpace(item.Artist)) != wantSinger {
			continue
		}
		return item, true
	}
	return MusicItem{}, false
}

func searchCacheMusic(query string) []MusicItem {
	query = strings.TrimSpace(query)
	if query == "" {
		return []MusicItem{}
	}
	results := make([]MusicItem, 0, 20)
	for _, folder := range listCacheFolders() {
		item, ok := buildCacheMusicItem(folder)
		if !ok {
			continue
		}
		if !containsIgnoreCase(folder, query) && !containsIgnoreCase(item.Title, query) && !containsIgnoreCase(item.Artist, query) {
			continue
		}
		results = append(results, item)
		if len(results) >= 20 {
			break
		}
	}
	return results
}

func getSourceStrategy() string {
	cfg := readCacheConfig()
	strategy := strings.ToLower(strings.TrimSpace(cfg.SourceStrategy))
	switch strategy {
	case "cache", "local", "legacy", "youtube":
		return strategy
	default:
		return "cache"
	}
}

func requestLegacyDirect(song, singer string, autoCache bool) MusicItem {
	sources := []string{"kuwo", "netease", "migu", "baidu"}
	for _, source := range sources {
		var item MusicItem
		if autoCache {
			item = YuafengAPIResponseHandler(source, song, singer)
		} else {
			item = YuafengAPIResponseHandlerNoCache(source, song, singer)
			if item.Title != "" {
				item.FromCache = false
				if item.SourceType == "" {
					item.SourceType = source
				}
			}
		}
		if item.Title != "" {
			return item
		}
	}
	return MusicItem{}
}

func resolveMusicByStrategy(song, singer string) MusicItem {
	strategy := getSourceStrategy()
	cfg := readCacheConfig()
	switch strategy {
	case "local":
		if item := getLocalMusicItem(song, singer); item.Title != "" {
			return item
		}
		if item, ok := findExactCacheMusic(song, singer); ok {
			return item
		}
		if item := requestLegacyDirect(song, singer, cfg.AutoCache); item.Title != "" {
			return item
		}
		if item := requestAndCacheMusicFromYouTube(song, singer); item.Title != "" {
			return item
		}
		return MusicItem{}
	case "legacy":
		if item := requestLegacyDirect(song, singer, cfg.AutoCache); item.Title != "" {
			return item
		}
		if item, ok := findExactCacheMusic(song, singer); ok {
			return item
		}
		if item := getLocalMusicItem(song, singer); item.Title != "" {
			return item
		}
		if item := requestAndCacheMusicFromYouTube(song, singer); item.Title != "" {
			return item
		}
		return MusicItem{}
	case "youtube":
		if item := requestAndCacheMusicFromYouTube(song, singer); item.Title != "" {
			return item
		}
		if item, ok := findExactCacheMusic(song, singer); ok {
			return item
		}
		if item := getLocalMusicItem(song, singer); item.Title != "" {
			return item
		}
		if item := requestLegacyDirect(song, singer, cfg.AutoCache); item.Title != "" {
			return item
		}
		return MusicItem{}
	case "cache":
		fallthrough
	default:
		if item, ok := findExactCacheMusic(song, singer); ok {
			return item
		}
		if item := getLocalMusicItem(song, singer); item.Title != "" {
			return item
		}
		if item := requestLegacyDirect(song, singer, cfg.AutoCache); item.Title != "" {
			return item
		}
		if item := requestAndCacheMusicFromYouTube(song, singer); item.Title != "" {
			return item
		}
		return MusicItem{}
	}
}

func appendSearchByStrategy(results *[]MusicItem, seen map[string]bool, items []MusicItem, limit int) {
	for _, item := range items {
		key := toLower(item.Title + "||" + item.Artist)
		if item.Title == "" || seen[key] {
			continue
		}
		seen[key] = true
		*results = append(*results, item)
		if len(*results) >= limit {
			return
		}
	}
}

func searchUnifiedByStrategy(query string, limit int) []MusicItem {
	results := make([]MusicItem, 0, limit)
	seen := map[string]bool{}
	strategy := getSourceStrategy()
	addCache := func() { appendSearchByStrategy(&results, seen, searchCacheMusic(query), limit) }
	addLocal := func() { appendSearchByStrategy(&results, seen, searchLocalMusic(query), limit) }
	addLegacy := func() {
		appendSearchByStrategy(&results, seen, searchFromSources(query), limit)
		if len(results) < limit {
			appendSearchByStrategy(&results, seen, searchFromAPI(query), limit)
		}
	}
	addYoutube := func() {
		item := requestAndCacheMusicFromYouTube(query, "")
		if item.Title != "" {
			appendSearchByStrategy(&results, seen, []MusicItem{item}, limit)
		}
	}
	switch strategy {
	case "local":
		addLocal()
		if len(results) < limit {
			addCache()
		}
		if len(results) < limit {
			addLegacy()
		}
		if len(results) < limit {
			addYoutube()
		}
	case "legacy":
		addLegacy()
		if len(results) < limit {
			addCache()
		}
		if len(results) < limit {
			addLocal()
		}
		if len(results) < limit {
			addYoutube()
		}
	case "youtube":
		addYoutube()
		if len(results) < limit {
			addCache()
		}
		if len(results) < limit {
			addLocal()
		}
		if len(results) < limit {
			addLegacy()
		}
	default:
		addCache()
		if len(results) < limit {
			addLocal()
		}
		if len(results) < limit {
			addLegacy()
		}
		if len(results) < limit {
			addYoutube()
		}
	}
	return results
}

// Request and cache music from API
func requestAndCacheMusic(song, singer string) MusicItem {
	cfg := readCacheConfig()
	if !cfg.AutoCache {
		return requestMusicNoCache(song, singer)
	}
	if item := requestLegacyDirect(song, singer, true); item.Title != "" {
		return item
	}

	song = strings.TrimSpace(song)
	singer = strings.TrimSpace(singer)
	if song == "" {
		return MusicItem{}
	}

	if singer == "" {
		for _, sep := range []string{" - ", "-", " / ", "/", "  ", " "} {
			parts := strings.SplitN(song, sep, 2)
			if len(parts) == 2 {
				left := strings.TrimSpace(parts[0])
				right := strings.TrimSpace(parts[1])
				if left != "" && right != "" {
					item := requestAndCacheMusicFromYouTube(left, right)
					if item.Title != "" {
						return item
					}
				}
			}
		}
	}

	item := requestAndCacheMusicFromYouTube(song, singer)
	if item.Title != "" {
		return item
	}
	return MusicItem{}
}

// 直接从远程URL流式转码（边下载边转码，超快！）
func streamConvertAudio(inputURL, outputFile string) error {
	fmt.Printf("[Info] Stream converting from URL to stable mp3 file\n")

	tempFile := outputFile + ".tmp"

	cmd := exec.Command("ffmpeg", "-y",
		"-i", inputURL,
		"-vn",
		"-ac", "1",
		"-ar", "24000",
		"-codec:a", "libmp3lame",
		"-b:a", "32k",
		"-write_xing", "0",
		"-id3v2_version", "0",
		tempFile)

	err := cmd.Run()
	if err != nil {
		fmt.Printf("[Error] Stream convert failed: %v\n", err)
		os.Remove(tempFile)
		return err
	}

	fileInfo, err := os.Stat(tempFile)
	if err != nil || fileInfo.Size() < 1024 {
		fmt.Printf("[Error] Converted file too small or empty\n")
		os.Remove(tempFile)
		return fmt.Errorf("converted file is too small")
	}

	err = os.Rename(tempFile, outputFile)
	if err != nil {
		fmt.Printf("[Error] Failed to rename temp file: %v\n", err)
		return err
	}

	fmt.Printf("[Success] Stable mp3 generated: %s\n", outputFile)
	return nil
}

// 实时流式转码到 HTTP Writer（边下载边播放！）
func streamConvertToWriter(inputURL string, w http.ResponseWriter) error {
	fmt.Printf("[Info] Live streaming from URL: %s\n", inputURL)

	// ffmpeg 边下载边转码，输出到 stdout
	cmd := exec.Command("ffmpeg",
		"-i", inputURL,
		"-threads", "0",
		"-ac", "1", "-ar", "24000", "-b:a", "32k", "-q:a", "9",
		"-f", "mp3",
		"-map_metadata", "-1",
		"pipe:1") // 输出到 stdout

	// 获取 stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %v", err)
	}

	// 启动 ffmpeg
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %v", err)
	}

	// 设置响应头
	w.Header().Set("Content-Type", "audio/mpeg")
	// 移除 Transfer-Encoding: chunked，让 Go 自动处理

	// 边读边写到 HTTP response
	buf := make([]byte, 8192)
	for {
		n, err := stdout.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			if f, ok := w.(http.Flusher); ok {
				f.Flush() // 立即发送给客户端
			}
		}
		if err != nil {
			break
		}
	}

	cmd.Wait()
	fmt.Printf("[Success] Live streaming completed\n")
	return nil
}

// Helper function for identifying file formats
func getMusicFileExtension(url string) (string, error) {
	resp, err := http.Head(url)
	if err != nil {
		return "", err
	}
	// Get file format from Content-Type header
	contentType := resp.Header.Get("Content-Type")
	ext, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", err
	}
	// Identify file extension based on file format
	switch ext {
	case "audio/mpeg":
		return ".mp3", nil
	case "audio/flac":
		return ".flac", nil
	case "audio/x-flac":
		return ".flac", nil
	case "audio/wav":
		return ".wav", nil
	case "audio/aac":
		return ".aac", nil
	case "audio/ogg":
		return ".ogg", nil
	case "application/octet-stream":
		// Try to guess file format from URL or other information
		if strings.Contains(url, ".mp3") {
			return ".mp3", nil
		} else if strings.Contains(url, ".flac") {
			return ".flac", nil
		} else if strings.Contains(url, ".wav") {
			return ".wav", nil
		} else if strings.Contains(url, ".aac") {
			return ".aac", nil
		} else if strings.Contains(url, ".ogg") {
			return ".ogg", nil
		} else {
			return "", fmt.Errorf("unknown file format from Content-Type and URL: %s", contentType)
		}
	default:
		return "", fmt.Errorf("unknown file format: %s", ext)
	}
}

// Helper function for identifying file formats
func GetDuration(filePath string) int {
	fmt.Printf("[Info] Get duration of obtaining music file %s\n", filePath)
	// Use ffprobe to get audio duration
	output, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", filePath).Output()
	if err != nil {
		fmt.Println("[Error] Error getting audio duration:", err)
		return 0
	}

	duration, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		fmt.Println("[Error] Error converting duration to float:", err)
		return 0
	}

	return int(duration)
}

// Helper function to compress and segment audio file
func compressAndSegmentAudio(inputFile, outputDir string) error {
	fmt.Printf("[Info] Compress and normalize audio file %s\n", inputFile)
	outputFile := filepath.Join(outputDir, "music.mp3")
	cmd := exec.Command("ffmpeg", "-y",
		"-i", inputFile,
		"-vn",
		"-ac", "1",
		"-ar", "24000",
		"-codec:a", "libmp3lame",
		"-b:a", "32k",
		"-write_xing", "0",
		"-id3v2_version", "0",
		outputFile)
	return cmd.Run()
}

// Helper function to obtain music data from local folder
func getLocalMusicItem(song, singer string) MusicItem {
	musicDir := "./files/music"
	fmt.Println("[Info] Reading local folder music.")
	files, err := os.ReadDir(musicDir)
	if err != nil {
		fmt.Println("[Error] Failed to read local music directory:", err)
		return MusicItem{}
	}

	normalizedSong := toLower(strings.TrimSpace(song))
	normalizedSinger := toLower(strings.TrimSpace(singer))
	supported := map[string]bool{".mp3": true, ".wav": true, ".flac": true, ".aac": true, ".ogg": true, ".m4a": true}

	for _, file := range files {
		name := file.Name()
		if file.IsDir() {
			match := strings.Contains(toLower(name), normalizedSong)
			if normalizedSinger != "" {
				match = match && strings.Contains(toLower(name), normalizedSinger)
			}
			if !match {
				continue
			}
			dirPath := filepath.Join(musicDir, name)
			parts := strings.SplitN(name, "-", 2)
			var artist, title string
			if len(parts) == 2 {
				artist = parts[0]
				title = parts[1]
			} else {
				title = name
			}
			basePath := "/cache/music/" + url.QueryEscape(name)
			return MusicItem{
				Title:        title,
				Artist:       artist,
				Filename:     name + ".mp3",
				CoverURL:     basePath + "/cover.jpg",
				LyricURL:     basePath + "/lyric.lrc",
				AudioFullURL: basePath + "/music.mp3",
				AudioURL:     basePath + "/music.mp3",
				M3U8URL:      basePath + "/music.m3u8",
				Duration:     GetDuration(filepath.Join(dirPath, "music.mp3")),
				FromCache:    true,
				SourceType:   "legacy_local_dir",
			}
		}

		ext := strings.ToLower(filepath.Ext(name))
		if !supported[ext] {
			continue
		}
		base := strings.TrimSuffix(name, ext)
		artist := "本地上传"
		title := base
		if strings.Contains(base, "-") {
			parts := strings.SplitN(base, "-", 2)
			artist = strings.TrimSpace(parts[0])
			title = strings.TrimSpace(parts[1])
		}
		match := strings.Contains(toLower(title), normalizedSong) || strings.Contains(toLower(base), normalizedSong)
		if normalizedSinger != "" {
			match = match && (strings.Contains(toLower(artist), normalizedSinger) || strings.Contains(toLower(base), normalizedSinger))
		}
		if !match {
			continue
		}
		basePath := "/music/" + url.PathEscape(name)
		return MusicItem{
			Title:        title,
			Artist:       artist,
			Filename:     name,
			AudioFullURL: basePath,
			AudioURL:     basePath,
			Duration:     GetDuration(filepath.Join(musicDir, name)),
			FromCache:    false,
			SourceType:   "local",
		}
	}
	return MusicItem{}
}

func requestMusicNoCache(song, singer string) MusicItem {
	if item := requestLegacyDirect(song, singer, false); item.Title != "" {
		return item
	}

	item := requestAndCacheMusicFromYouTube(song, singer)
	if item.Title != "" {
		return item
	}
	return MusicItem{}
}
