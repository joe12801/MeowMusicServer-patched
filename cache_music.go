package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type CacheConfig struct {
	AutoCache      bool   `json:"auto_cache"`
	SourceStrategy string `json:"source_strategy"`
}

type CacheMusicActionRequest struct {
	Folder   string `json:"folder"`
	Filename string `json:"filename"`
}

func cacheConfigPath() string {
	return "./files/cache_config.json"
}

func normalizeSourceStrategy(strategy string) string {
	strategy = strings.ToLower(strings.TrimSpace(strategy))
	switch strategy {
	case "cache", "local", "legacy", "youtube":
		return strategy
	default:
		return "cache"
	}
}

func readCacheConfig() CacheConfig {
	cfg := CacheConfig{AutoCache: TrueDefault(), SourceStrategy: "cache"}
	b, err := os.ReadFile(cacheConfigPath())
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(b, &cfg)
	cfg.SourceStrategy = normalizeSourceStrategy(cfg.SourceStrategy)
	return cfg
}

func TrueDefault() bool { return true }

func writeCacheConfig(cfg CacheConfig) error {
	cfg.SourceStrategy = normalizeSourceStrategy(cfg.SourceStrategy)
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
		cfg := readCacheConfig()
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if err := writeCacheConfig(cfg); err != nil {
			http.Error(w, "Save failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "message": "设置已保存", "config": readCacheConfig()})
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func HandleCacheMusicList(w http.ResponseWriter, r *http.Request) {
	var result []map[string]interface{}
	for _, folder := range listCacheFolders() {
		sourceType, artist, title, leaf := parseCacheFolder(folder)
		base := cacheBaseURL(folder) + "/music.mp3"
		result = append(result, map[string]interface{}{
			"folder":         folder,
			"filename":       leaf + ".mp3",
			"title":          title,
			"artist":         artist,
			"audio_url":      base,
			"audio_full_url": base,
			"from_cache":     true,
			"source_type":    sourceType,
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
	folder := sanitizeCacheFolderPath(req.Folder)
	if folder == "" {
		folder = sanitizeCacheFolderPath(strings.TrimSuffix(req.Filename, filepath.Ext(req.Filename)))
	}
	folder = resolveCacheFolder(folder)
	if folder == "" {
		http.Error(w, "Invalid folder", http.StatusBadRequest)
		return
	}
	src := cacheFilePath(folder, "music.mp3")
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
	folder := sanitizeCacheFolderPath(req.Folder)
	if folder == "" {
		folder = sanitizeCacheFolderPath(strings.TrimSuffix(req.Filename, filepath.Ext(req.Filename)))
	}
	folder = resolveCacheFolder(folder)
	if folder == "" {
		http.Error(w, "Invalid folder", http.StatusBadRequest)
		return
	}
	if err := os.RemoveAll(cacheDirPath(folder)); err != nil {
		http.Error(w, "Delete failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "缓存已删除"})
}
