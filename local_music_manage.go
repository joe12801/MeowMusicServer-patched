package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type LocalMusicActionRequest struct {
	Filename    string `json:"filename"`
	NewFilename string `json:"new_filename"`
}

func sanitizeLocalFilename(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ReplaceAll(name, "*", "_")
	name = strings.ReplaceAll(name, "?", "_")
	name = strings.ReplaceAll(name, "\"", "_")
	name = strings.ReplaceAll(name, "<", "_")
	name = strings.ReplaceAll(name, ">", "_")
	name = strings.ReplaceAll(name, "|", "_")
	return name
}

func HandleDeleteLocalMusic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req LocalMusicActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	filename := sanitizeLocalFilename(req.Filename)
	if filename == "" {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}
	fullPath := filepath.Join("./files/music", filename)
	if err := os.Remove(fullPath); err != nil {
		http.Error(w, "Delete failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "删除成功"})
}

func HandleRenameLocalMusic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req LocalMusicActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	oldName := sanitizeLocalFilename(req.Filename)
	newName := sanitizeLocalFilename(req.NewFilename)
	if oldName == "" || newName == "" {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}
	if filepath.Ext(newName) == "" {
		newName += filepath.Ext(oldName)
	}
	oldPath := filepath.Join("./files/music", oldName)
	newPath := filepath.Join("./files/music", newName)
	if err := os.Rename(oldPath, newPath); err != nil {
		http.Error(w, "Rename failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "重命名成功", "filename": newName})
}
