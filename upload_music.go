package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func sanitizeUploadName(name string) string {
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

func HandleUploadMusic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(200 << 20); err != nil {
		http.Error(w, "Failed to parse multipart form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	filename := sanitizeUploadName(header.Filename)
	if filename == "" {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	ext := strings.ToLower(filepath.Ext(filename))
	allowed := map[string]bool{
		".mp3":  true,
		".wav":  true,
		".flac": true,
		".aac":  true,
		".ogg":  true,
		".m4a":  true,
	}
	if !allowed[ext] {
		http.Error(w, "Unsupported file type", http.StatusBadRequest)
		return
	}

	musicDir := "./files/music"
	if err := os.MkdirAll(musicDir, 0755); err != nil {
		http.Error(w, "Failed to create music directory", http.StatusInternalServerError)
		return
	}

	targetPath := filepath.Join(musicDir, filename)
	out, err := os.Create(targetPath)
	if err != nil {
		http.Error(w, "Failed to create target file", http.StatusInternalServerError)
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	fmt.Printf("[Upload] music uploaded: %s\n", targetPath)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "success",
		"message":  "上传成功",
		"filename": filename,
		"path":     targetPath,
	})
}
