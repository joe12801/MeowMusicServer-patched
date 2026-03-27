package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// SearchResult represents a search result with frontend-compatible fields
type SearchResult struct {
	Title        string `json:"title"`
	Artist       string `json:"artist"`
	URL          string `json:"url"`  // For frontend compatibility
	Link         string `json:"link"` // Alternative field
	AudioURL     string `json:"audio_url"`
	AudioFullURL string `json:"audio_full_url"`
	M3U8URL      string `json:"m3u8_url"`
	LyricURL     string `json:"lyric_url"` // Lyrics URL
	CoverURL     string `json:"cover_url"`
	Duration     int    `json:"duration"` // Duration in seconds
	Filename     string `json:"filename,omitempty"`
	FromCache    bool   `json:"from_cache"`
	SourceType   string `json:"source_type,omitempty"`
}

// HandleSearch handles music search requests
func HandleSearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	query := r.URL.Query().Get("query")
	fmt.Printf("[API] Search request: query=%s\n", query)

	if query == "" {
		json.NewEncoder(w).Encode([]SearchResult{})
		return
	}

	results := searchAllResults(query, r)
	fmt.Printf("[API] Search completed: %d results found\n", len(results))
	json.NewEncoder(w).Encode(results)
}

func searchAllResults(query string, r *http.Request) []SearchResult {
	results := make([]SearchResult, 0, 20)
	seen := make(map[string]bool)

	appendUnique := func(items []SearchResult) {
		for _, item := range items {
			if item.Title == "" {
				continue
			}
			key := toLower(item.Title + "||" + item.Artist)
			if seen[key] {
				continue
			}
			seen[key] = true
			results = append(results, item)
			fmt.Printf("[API] Result %d: title=%s, artist=%s, url=%s, lyric=%s\n", len(results), item.Title, item.Artist, item.URL, item.LyricURL)
			if len(results) >= 20 {
				return
			}
		}
	}

	// 搜索优先级：缓存 > 本地 > 旧源 > YouTube 补源 > 小爱音箱远端
	appendUnique(convertMusicItemsToSearchResults(searchCacheMusic(query)))
	if len(results) < 20 {
		appendUnique(convertMusicItemsToSearchResults(searchLocalMusic(query)))
	}
	if len(results) < 20 {
		appendUnique(convertMusicItemsToSearchResults(searchFromSources(query)))
	}
	if len(results) < 20 {
		appendUnique(convertMusicItemsToSearchResults(searchFromAPI(query)))
	}

	if len(results) > 20 {
		results = results[:20]
	}
	return results
}

func convertMusicItemsToSearchResults(items []MusicItem) []SearchResult {
	results := make([]SearchResult, 0, len(items))
	for _, item := range items {
		audioURL := item.AudioFullURL
		if audioURL == "" {
			audioURL = item.AudioURL
		}
		results = append(results, SearchResult{
			Title:        item.Title,
			Artist:       item.Artist,
			URL:          audioURL,
			Link:         audioURL,
			AudioURL:     item.AudioURL,
			AudioFullURL: audioURL,
			M3U8URL:      item.M3U8URL,
			LyricURL:     item.LyricURL,
			CoverURL:     item.CoverURL,
			Duration:     item.Duration,
			Filename:     item.Filename,
			FromCache:    item.FromCache,
			SourceType:   item.SourceType,
		})
	}
	return results
}

func searchFromSources(query string) []MusicItem {
	var results []MusicItem
	sources := readSources()
	for _, source := range sources {
		if containsIgnoreCase(source.Title, query) || containsIgnoreCase(source.Artist, query) {
			item := MusicItem{
				Title:        source.Title,
				Artist:       source.Artist,
				AudioURL:     source.AudioURL,
				AudioFullURL: source.AudioFullURL,
				M3U8URL:      source.M3U8URL,
				LyricURL:     source.LyricURL,
				CoverURL:     source.CoverURL,
				Duration:     source.Duration,
				FromCache:    false,
			}
			results = append(results, item)
			if len(results) >= 20 {
				break
			}
		}
	}
	return results
}

// searchLocalMusic searches music from local files
func searchLocalMusic(query string) []MusicItem {
	query = strings.TrimSpace(query)
	if query == "" {
		return []MusicItem{}
	}
	musicDir := "./files/music"
	entries, err := os.ReadDir(musicDir)
	if err != nil {
		return []MusicItem{}
	}
	supported := map[string]bool{".mp3": true, ".wav": true, ".flac": true, ".aac": true, ".ogg": true, ".m4a": true}
	results := make([]MusicItem, 0, 20)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
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
		if !containsIgnoreCase(base, query) && !containsIgnoreCase(title, query) && !containsIgnoreCase(artist, query) {
			continue
		}
		urlPath := "/music/" + url.PathEscape(name)
		results = append(results, MusicItem{Title: title, Artist: artist, Filename: name, AudioURL: urlPath, AudioFullURL: urlPath, Duration: GetDuration(filepath.Join(musicDir, name)), FromCache: false, SourceType: "local"})
		if len(results) >= 20 {
			break
		}
	}
	return results
}

// searchFromAPI searches music from external APIs
func searchFromAPI(query string) []MusicItem {
	var results []MusicItem
	item := requestAndCacheMusic(query, "")
	if item.Title != "" {
		results = append(results, item)
	}
	return results
}

// containsIgnoreCase checks if a string contains substring (case insensitive)
func containsIgnoreCase(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	return contains(s, substr)
}

func toLower(s string) string {
	var result []rune
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			r += 32
		}
		result = append(result, r)
	}
	return string(result)
}

func contains(s, substr string) bool {
	return len(substr) == 0 || indexString(s, substr) >= 0
}

func indexString(s, substr string) int {
	n := len(substr)
	if n == 0 {
		return 0
	}
	for i := 0; i+n <= len(s); i++ {
		if s[i:i+n] == substr {
			return i
		}
	}
	return -1
}
