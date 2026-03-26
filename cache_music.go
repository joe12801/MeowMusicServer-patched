package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type CacheConfig struct {
	AutoCache bool `json:"auto_cache"`
}

type CacheMusicActionRequest struct {
	Folder   string `json:"folder"`
	Filename string `json:"filename"`
}

func cacheConfigPath() string {
	return "./files/cache_config.json"
}

func readCacheConfig() CacheConfig {
	cfg := CacheConfig{AutoCache: TrueDefault()}
	b, err := os.ReadFile(cacheConfigPath())
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(b, &cfg)
	return cfg
}

func TrueDefault() bool { return true }

func writeCacheConfig(cfg CacheConfig) error {
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cacheConfigPath(), b, 0644)
}

func HandleCacheConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(readCacheConfig())
		return
	}
	if r.Method == http.MethodPost {
		var cfg CacheConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if err := writeCacheConfig(cfg); err != nil {
			http.Error(w, "Save failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "设置已保存"})
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func HandleCacheMusicList(w http.ResponseWriter, r *http.Request) {
	cacheDir := "./files/cache/music"
	_ = os.MkdirAll(cacheDir, 0755)
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		http.Error(w, "Failed to read cache music directory", http.StatusInternalServerError)
		return
	}
	var result []map[string]interface{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		folder := entry.Name()
		mp3Path := filepath.Join(cacheDir, folder, "music.mp3")
		if _, err := os.Stat(mp3Path); err != nil {
			continue
		}
		title := folder
		artist := "缓存音乐"
		if strings.Contains(folder, "-") {
			parts := strings.SplitN(folder, "-", 2)
			artist = strings.TrimSpace(parts[0])
			title = strings.TrimSpace(parts[1])
		}
		base := "/cache/music/" + url.PathEscape(folder) + "/music.mp3"
		result = append(result, map[string]interface{}{
			"folder":         folder,
			"filename":       folder + ".mp3",
			"title":          title,
			"artist":         artist,
			"audio_url":      base,
			"audio_full_url": base,
			"from_cache":     true,
			"source_type":    "cache",
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func HandlePromoteCacheMusic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req CacheMusicActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	folder := sanitizeLocalFilename(req.Folder)
	if folder == "" {
		folder = sanitizeLocalFilename(strings.TrimSuffix(req.Filename, filepath.Ext(req.Filename)))
	}
	if folder == "" {
		http.Error(w, "Invalid folder", http.StatusBadRequest)
		return
	}
	src := filepath.Join("./files/cache/music", folder, "music.mp3")
	dstDir := "./files/music"
	_ = os.MkdirAll(dstDir, 0755)
	dst := filepath.Join(dstDir, folder+".mp3")
	in, err := os.ReadFile(src)
	if err != nil {
		http.Error(w, "Read cache failed", http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(dst, in, 0644); err != nil {
		http.Error(w, "Write local failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "已转入本地音乐"})
}

func HandleDeleteCacheMusic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req CacheMusicActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	folder := sanitizeLocalFilename(req.Folder)
	if folder == "" {
		folder = sanitizeLocalFilename(strings.TrimSuffix(req.Filename, filepath.Ext(req.Filename)))
	}
	if folder == "" {
		http.Error(w, "Invalid folder", http.StatusBadRequest)
		return
	}
	if err := os.RemoveAll(filepath.Join("./files/cache/music", folder)); err != nil {
		http.Error(w, "Delete failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "缓存已删除"})
}
