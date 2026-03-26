package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type ytSearchEntry struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Uploader  string  `json:"uploader"`
	Duration  float64 `json:"duration"`
	Thumbnail string  `json:"thumbnail"`
}

func sanitizeFileNamePart(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	re := regexp.MustCompile(`[\\/:*?"<>|]+`)
	s = re.ReplaceAllString(s, "_")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	return s
}

func searchYouTubeTopMV(query string) (*ytSearchEntry, error) {
	searchQuery := fmt.Sprintf("ytsearch5:%s MV 官方版", query)
	cmd := exec.Command("yt-dlp",
		"--dump-single-json",
		"--flat-playlist",
		"--no-warnings",
		searchQuery,
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var payload struct {
		Entries []ytSearchEntry `json:"entries"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil, err
	}
	if len(payload.Entries) == 0 {
		return nil, fmt.Errorf("no youtube results")
	}
	return &payload.Entries[0], nil
}

func requestAndCacheMusicFromYouTube(song, singer string) MusicItem {
	query := strings.TrimSpace(strings.TrimSpace(song) + " " + strings.TrimSpace(singer))
	if query == "" {
		return MusicItem{}
	}

	entry, err := searchYouTubeTopMV(query)
	if err != nil {
		fmt.Printf("[YouTube] search failed: %v\n", err)
		return MusicItem{}
	}

	artist := singer
	if strings.TrimSpace(artist) == "" {
		artist = entry.Uploader
	}
	artist = sanitizeFileNamePart(artist)
	title := sanitizeFileNamePart(song)
	if strings.TrimSpace(title) == "" {
		title = sanitizeFileNamePart(entry.Title)
	}

	dirName := filepath.Join("./files/cache/music", artist+"-"+title)
	if err := os.MkdirAll(dirName, 0755); err != nil {
		fmt.Printf("[YouTube] mkdir failed: %v\n", err)
		return MusicItem{}
	}

	outputTemplate := filepath.Join(dirName, "music.%(ext)s")
	videoURL := "https://www.youtube.com/watch?v=" + entry.ID
	args := []string{
		"--no-playlist",
		"-x",
		"--audio-format", "mp3",
		"--audio-quality", "0",
		"--extractor-args", "youtube:player_client=tv,web;formats=missing_pot",
		"--extractor-args", "youtube:player_skip=webpage,configs",
		"-o", outputTemplate,
	}
	cookiePath := filepath.Join(".", "youtube-cookies.txt")
	if _, err := os.Stat(cookiePath); err == nil {
		args = append(args, "--cookies", cookiePath)
	}
	args = append(args, videoURL)
	cmd := exec.Command("yt-dlp", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("[YouTube] download failed: %v\n%s\n", err, string(out))
		return MusicItem{}
	}

	musicPath := filepath.Join(dirName, "music.mp3")
	if _, err := os.Stat(musicPath); err != nil {
		fmt.Printf("[YouTube] mp3 not found after download: %v\n", err)
		return MusicItem{}
	}

	coverURL := ""
	if strings.TrimSpace(entry.Thumbnail) != "" {
		coverURL = entry.Thumbnail
	}

	basePath := "/cache/music/" + artist + "-" + title
	return MusicItem{
		Title:        title,
		Artist:       artist,
		Filename:     artist + "-" + title + ".mp3",
		CoverURL:     coverURL,
		LyricURL:     "",
		AudioFullURL: basePath + "/music.mp3",
		AudioURL:     basePath + "/music.mp3",
		M3U8URL:      "",
		Duration:     int(entry.Duration),
		FromCache:    true,
		SourceType:   "youtube",
	}
}
