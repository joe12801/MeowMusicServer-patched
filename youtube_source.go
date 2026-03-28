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

func findDownloadedYouTubeSource(dirName string) string {
	candidates := []string{"source.m4a", "source.webm", "source.mp4", "source.mp3", "source.opus", "source.aac"}
	for _, name := range candidates {
		path := filepath.Join(dirName, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() && info.Size() > 1024 {
			return path
		}
	}
	matches, _ := filepath.Glob(filepath.Join(dirName, "source.*"))
	for _, path := range matches {
		if info, err := os.Stat(path); err == nil && !info.IsDir() && info.Size() > 1024 {
			return path
		}
	}
	return ""
}

func transcodeToXiaozhiMP3(inputPath, outputPath string, bitrate string) error {
	if strings.TrimSpace(bitrate) == "" {
		bitrate = "48k"
	}
	tempPath := outputPath + ".tmp"
	_ = os.Remove(tempPath)
	cmd := exec.Command("ffmpeg",
		"-y",
		"-i", inputPath,
		"-vn",
		"-ac", "1",
		"-ar", "24000",
		"-codec:a", "libmp3lame",
		"-b:a", bitrate,
		"-write_xing", "0",
		"-id3v2_version", "0",
		tempPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("ffmpeg transcode failed: %v %s", err, string(out))
	}
	info, err := os.Stat(tempPath)
	if err != nil || info.Size() < 1024 {
		_ = os.Remove(tempPath)
		return fmt.Errorf("transcoded mp3 missing or too small")
	}
	_ = os.Remove(outputPath)
	return os.Rename(tempPath, outputPath)
}

func normalizeCachedMusicToXiaozhi(dirName string) error {
	musicPath := filepath.Join(dirName, "music.mp3")
	if info, err := os.Stat(musicPath); err == nil && !info.IsDir() && info.Size() > 1024 {
		if err := transcodeToXiaozhiMP3(musicPath, musicPath, "48k"); err == nil {
			return nil
		} else {
			fmt.Printf("[YouTube] normalize existing mp3 with 48k failed: %v\n", err)
			if err := transcodeToXiaozhiMP3(musicPath, musicPath, "32k"); err == nil {
				return nil
			}
		}
	}

	sourcePath := findDownloadedYouTubeSource(dirName)
	if sourcePath == "" {
		return fmt.Errorf("downloaded source file not found")
	}
	if err := transcodeToXiaozhiMP3(sourcePath, musicPath, "48k"); err == nil {
		return nil
	}
	return transcodeToXiaozhiMP3(sourcePath, musicPath, "32k")
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

	folder := cacheFolderBySource("youtube", artist, title)
	dirName := cacheDirPath(folder)
	if err := os.MkdirAll(dirName, 0755); err != nil {
		fmt.Printf("[YouTube] mkdir failed: %v\n", err)
		return MusicItem{}
	}

	musicPath := filepath.Join(dirName, "music.mp3")
	if info, err := os.Stat(musicPath); err == nil && !info.IsDir() && info.Size() > 1024 {
		if err := normalizeCachedMusicToXiaozhi(dirName); err != nil {
			fmt.Printf("[YouTube] normalize cached mp3 failed: %v\n", err)
		} else {
			fmt.Printf("[YouTube] cache hit and normalized: %s\n", musicPath)
		}
	} else {
		outputTemplate := filepath.Join(dirName, "source.%(ext)s")
		videoURL := "https://www.youtube.com/watch?v=" + entry.ID
		args := []string{
			"--no-playlist",
			"-f", "bestaudio/best",
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
		if err := normalizeCachedMusicToXiaozhi(dirName); err != nil {
			fmt.Printf("[YouTube] mp3 normalize failed: %v\n", err)
			return MusicItem{}
		}
	}

	if _, err := os.Stat(musicPath); err != nil {
		fmt.Printf("[YouTube] mp3 not found after normalize: %v\n", err)
		return MusicItem{}
	}

	coverURL := ""
	if strings.TrimSpace(entry.Thumbnail) != "" {
		coverURL = entry.Thumbnail
	}

	basePath := cacheBaseURL(folder)
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
