package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func buildLocalMusicItems() []map[string]interface{} {
	musicDir := "./files/music"
	_ = os.MkdirAll(musicDir, 0755)

	entries, err := os.ReadDir(musicDir)
	if err != nil {
		return []map[string]interface{}{}
	}

	supported := map[string]bool{".mp3": true, ".wav": true, ".flac": true, ".aac": true, ".ogg": true, ".m4a": true}
	result := make([]map[string]interface{}, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if !supported[ext] {
			continue
		}

		title := strings.TrimSuffix(name, ext)
		artist := "本地上传"
		if strings.Contains(title, "-") {
			parts := strings.SplitN(title, "-", 2)
			artist = strings.TrimSpace(parts[0])
			title = strings.TrimSpace(parts[1])
		}

		urlPath := "/music/" + url.PathEscape(name)
		result = append(result, map[string]interface{}{
			"filename":       name,
			"title":          title,
			"artist":         artist,
			"audio_url":      urlPath,
			"audio_full_url": urlPath,
			"m3u8_url":       "",
			"lyric_url":      "",
			"cover_url":      "",
			"duration":       0,
			"from_cache":     false,
			"source_type":    "local",
		})
	}
	return result
}

func HandleLocalMusicList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(buildLocalMusicItems())
}
