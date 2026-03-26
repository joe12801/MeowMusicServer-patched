package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func youtubeCookiePath() string {
	return filepath.Join(".", "youtube-cookies.txt")
}

func youtubeCookieStatus() map[string]interface{} {
	path := youtubeCookiePath()
	info, err := os.Stat(path)
	if err != nil {
		return map[string]interface{}{
			"exists": false,
			"path":   path,
		}
	}
	return map[string]interface{}{
		"exists":        true,
		"path":          path,
		"size":          info.Size(),
		"modified_unix": info.ModTime().Unix(),
		"modified_at":   info.ModTime().Format(time.RFC3339),
	}
}

func HandleYouTubeCookieStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"cookie": youtubeCookieStatus(),
	})
}

func HandleYouTubeCookieUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	contentType := r.Header.Get("Content-Type")
	var raw []byte
	var err error

	if strings.Contains(contentType, "application/json") {
		var req struct {
			Content string `json:"content"`
		}
		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			http.Error(w, "Error reading request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		raw = []byte(req.Content)
	} else {
		raw, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()
	}

	content := strings.TrimSpace(string(raw))
	if content == "" {
		http.Error(w, "Empty cookie content", http.StatusBadRequest)
		return
	}
	if !strings.Contains(content, "youtube.com") || !strings.Contains(content, "Netscape HTTP Cookie File") {
		http.Error(w, "Invalid YouTube cookie file", http.StatusBadRequest)
		return
	}

	path := youtubeCookiePath()
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content+"\n"), 0600); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write temp cookie file: %v", err), http.StatusInternalServerError)
		return
	}
	if err := os.Rename(tmpPath, path); err != nil {
		http.Error(w, fmt.Sprintf("Failed to replace cookie file: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Printf("[YouTube] cookie updated via API, size=%d\n", len(content))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"cookie": youtubeCookieStatus(),
	})
}
