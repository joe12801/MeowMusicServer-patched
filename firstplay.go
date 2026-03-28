package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const firstPlayPreviewDurationSeconds = "45"

func buildFirstPlayURL(song, singer string) string {
	q := url.Values{}
	q.Set("song", strings.TrimSpace(song))
	if strings.TrimSpace(singer) != "" {
		q.Set("singer", strings.TrimSpace(singer))
	}
	return "/api/stream-first?" + q.Encode()
}

func firstPlayPreviewPath(song, singer string) string {
	key := asyncCacheKey(song, singer)
	return filepath.Join("./files/cache/tasks", key, "firstplay.mp3")
}

func ensureFirstPlayPreviewAsync(song, singer, remoteURL string) {
	if strings.TrimSpace(remoteURL) == "" {
		return
	}
	previewPath := firstPlayPreviewPath(song, singer)
	if info, err := os.Stat(previewPath); err == nil && !info.IsDir() && info.Size() > 1024 {
		return
	}
	go func() {
		if err := os.MkdirAll(filepath.Dir(previewPath), 0755); err != nil {
			fmt.Printf("[FirstPlay] mkdir preview dir failed: %v\n", err)
			return
		}
		tmpPath := previewPath + ".tmp"
		_ = os.Remove(tmpPath)
		cmd := exec.Command("ffmpeg",
			"-y",
			"-i", remoteURL,
			"-vn",
			"-t", firstPlayPreviewDurationSeconds,
			"-ac", "1",
			"-ar", "24000",
			"-codec:a", "libmp3lame",
			"-b:a", "32k",
			"-write_xing", "0",
			"-id3v2_version", "0",
			tmpPath,
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			_ = os.Remove(tmpPath)
			fmt.Printf("[FirstPlay] generate preview failed: %v %s\n", err, string(out))
			return
		}
		if info, err := os.Stat(tmpPath); err != nil || info.Size() < 1024 {
			_ = os.Remove(tmpPath)
			fmt.Printf("[FirstPlay] preview too small or missing: %s\n", previewPath)
			return
		}
		_ = os.Remove(previewPath)
		if err := os.Rename(tmpPath, previewPath); err != nil {
			fmt.Printf("[FirstPlay] rename preview failed: %v\n", err)
			return
		}
		fmt.Printf("[FirstPlay] preview ready: %s\n", previewPath)
	}()
}

func resolveYouTubeDirectAudioURL(song, singer string) string {
	query := strings.TrimSpace(strings.TrimSpace(song) + " " + strings.TrimSpace(singer))
	if query == "" {
		return ""
	}
	entry, err := searchYouTubeTopMV(query)
	if err != nil || entry == nil || strings.TrimSpace(entry.ID) == "" {
		fmt.Printf("[FirstPlay] youtube search failed: %v\n", err)
		return ""
	}
	videoURL := "https://www.youtube.com/watch?v=" + entry.ID
	args := []string{
		"-g",
		"--no-playlist",
		"-f", "bestaudio/best",
		"--extractor-args", "youtube:player_client=tv,web;formats=missing_pot",
		"--extractor-args", "youtube:player_skip=webpage,configs",
	}
	cookiePath := filepath.Join(".", "youtube-cookies.txt")
	if _, err := os.Stat(cookiePath); err == nil {
		args = append(args, "--cookies", cookiePath)
	}
	args = append(args, videoURL)
	out, err := exec.Command("yt-dlp", args...).CombinedOutput()
	if err != nil {
		fmt.Printf("[FirstPlay] youtube direct audio url failed: %v %s\n", err, string(out))
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			return line
		}
	}
	return ""
}

func resolveRemoteAudioSource(song, singer string) string {
	if remoteURL := strings.TrimSpace(getRemoteMusicURLOnly(song, singer)); remoteURL != "" {
		fmt.Printf("[FirstPlay] using legacy remote source\n")
		return remoteURL
	}
	if remoteURL := strings.TrimSpace(resolveYouTubeDirectAudioURL(song, singer)); remoteURL != "" {
		fmt.Printf("[FirstPlay] using youtube fallback direct source\n")
		return remoteURL
	}
	return ""
}

func HandleFirstPlayStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Accept-Ranges", "bytes")

	song := strings.TrimSpace(r.URL.Query().Get("song"))
	singer := strings.TrimSpace(r.URL.Query().Get("singer"))
	if song == "" {
		http.Error(w, "Missing song parameter", http.StatusBadRequest)
		return
	}

	if item, ok := findExactCacheMusic(song, singer); ok {
		folder := strings.TrimPrefix(item.AudioURL, "/cache/music/")
		folder = strings.TrimSuffix(folder, "/music.mp3")
		folder = strings.TrimSpace(folder)
		if folder != "" {
			cachedFile := filepath.Join("./files/cache/music", folder, "music.mp3")
			if _, err := os.Stat(cachedFile); err == nil {
				fmt.Printf("[FirstPlay] serving stable cache: %s\n", cachedFile)
				w.Header().Set("Content-Type", "audio/mpeg")
				http.ServeFile(w, r, cachedFile)
				return
			}
		}
	}

	previewPath := firstPlayPreviewPath(song, singer)
	if info, err := os.Stat(previewPath); err == nil && !info.IsDir() && info.Size() > 1024 {
		fmt.Printf("[FirstPlay] serving warm preview: %s\n", previewPath)
		w.Header().Set("Content-Type", "audio/mpeg")
		http.ServeFile(w, r, previewPath)
		return
	}

	remoteURL := resolveRemoteAudioSource(song, singer)
	if remoteURL == "" {
		http.Error(w, "Failed to resolve remote audio source", http.StatusNotFound)
		return
	}

	ensureFirstPlayPreviewAsync(song, singer, remoteURL)
	go enqueueAsyncCacheTask(song, singer)

	fmt.Printf("[FirstPlay] live transcoding begin: %s - %s\n", singer, song)
	if err := streamConvertToWriter(remoteURL, w); err != nil {
		fmt.Printf("[FirstPlay] live transcoding failed: %v\n", err)
	}
}
