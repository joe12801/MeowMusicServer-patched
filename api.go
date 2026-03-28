package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// APIHandler handles API requests.
func apiHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Server", "MeowMusicEmbeddedServer")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	queryParams := r.URL.Query()
	fmt.Printf("[Web Access] Handling request for %s?%s\n", r.URL.Path, queryParams.Encode())
	song := strings.TrimSpace(queryParams.Get("song"))
	singer := strings.TrimSpace(queryParams.Get("singer"))

	ip, err := IPhandler(r)
	if err != nil {
		ip = "0.0.0.0"
	}

	if song == "" {
		musicItem := MusicItem{FromCache: false, IP: ip}
		json.NewEncoder(w).Encode(musicItem)
		return
	}

	var scheme string
	if r.TLS == nil {
		scheme = "http"
	} else {
		scheme = "https"
	}

	strategy := getSourceStrategy()
	fmt.Printf("[Play] Resolving with source strategy: %s\n", strategy)
	musicItem := resolveMusicByStrategy(song, singer)
	found := musicItem.Title != ""

	if !found {
		musicItem = MusicItem{FromCache: false, IP: ip}
		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(false)
		encoder.Encode(musicItem)
		return
	}

	if strings.HasPrefix(musicItem.AudioURL, "/") {
		musicItem.AudioURL = scheme + "://" + r.Host + musicItem.AudioURL
	}
	if strings.HasPrefix(musicItem.AudioFullURL, "/") {
		musicItem.AudioFullURL = scheme + "://" + r.Host + musicItem.AudioFullURL
	}
	if strings.HasPrefix(musicItem.M3U8URL, "/") {
		musicItem.M3U8URL = scheme + "://" + r.Host + musicItem.M3U8URL
	}
	if strings.HasPrefix(musicItem.LyricURL, "/") {
		musicItem.LyricURL = scheme + "://" + r.Host + musicItem.LyricURL
	}
	if strings.HasPrefix(musicItem.CoverURL, "/") {
		musicItem.CoverURL = scheme + "://" + r.Host + musicItem.CoverURL
	}
	musicItem.IP = ip

	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.Encode(musicItem)
}

// streamLiveHandler 保留作为调试/备用接口，不再作为默认设备播放入口
func streamLiveHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Accept-Ranges", "bytes")

	queryParams := r.URL.Query()
	song := queryParams.Get("song")
	singer := queryParams.Get("singer")

	fmt.Printf("[Stream Live] Request: song=%s, singer=%s\n", song, singer)
	if song == "" {
		http.Error(w, "Missing song parameter", http.StatusBadRequest)
		return
	}

	if item, ok := findExactCacheMusic(song, singer); ok {
		folder := strings.TrimPrefix(item.AudioURL, "/cache/music/")
		folder = strings.TrimSuffix(folder, "/music.mp3")
		folder, _ = url.PathUnescape(folder)
		cachedFile := cacheFilePath(folder, "music.mp3")
		if _, err := os.Stat(cachedFile); err == nil {
			fmt.Printf("[Stream Live] Serving from cache: %s\n", cachedFile)
			w.Header().Set("Content-Type", "audio/mpeg")
			http.ServeFile(w, r, cachedFile)
			return
		}
	}

	fmt.Printf("[Stream Live] Cache miss, fetching from API...\n")
	remoteURL := getRemoteMusicURLOnly(song, singer)
	if remoteURL == "" {
		http.Error(w, "Failed to get remote music URL", http.StatusNotFound)
		return
	}

	fmt.Printf("[Stream Live] Starting live stream from: %s\n", remoteURL)
	if err := streamConvertToWriter(remoteURL, w); err != nil {
		fmt.Printf("[Stream Live] Error: %v\n", err)
	}
}
