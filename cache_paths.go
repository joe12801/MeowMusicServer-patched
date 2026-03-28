package main

import (
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const cacheMusicRoot = "./files/cache/music"

func sanitizeCacheSegment(name string) string {
	name = sanitizeLocalFilename(name)
	name = strings.TrimSpace(name)
	if name == "" {
		return "unknown"
	}
	return name
}

func sanitizeCacheFolderPath(folder string) string {
	folder = strings.ReplaceAll(strings.TrimSpace(folder), "\\", "/")
	parts := strings.Split(folder, "/")
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == ".." {
			continue
		}
		part = sanitizeCacheSegment(part)
		if part != "" {
			clean = append(clean, part)
		}
	}
	return strings.Join(clean, "/")
}

func cacheLeafName(artist, title string) string {
	artist = sanitizeCacheSegment(artist)
	title = sanitizeCacheSegment(title)
	return artist + "-" + title
}

func cacheFolderBySource(source, artist, title string) string {
	source = sanitizeCacheSegment(source)
	if source == "" {
		source = "legacy"
	}
	return filepath.ToSlash(filepath.Join(source, cacheLeafName(artist, title)))
}

func cacheDirPath(folder string) string {
	folder = sanitizeCacheFolderPath(folder)
	if folder == "" {
		return cacheMusicRoot
	}
	return filepath.Join(cacheMusicRoot, filepath.FromSlash(folder))
}

func cacheFilePath(folder, name string) string {
	return filepath.Join(cacheDirPath(folder), name)
}

func cacheBaseURL(folder string) string {
	folder = sanitizeCacheFolderPath(folder)
	parts := strings.Split(folder, "/")
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		escaped = append(escaped, url.PathEscape(part))
	}
	return "/cache/music/" + strings.Join(escaped, "/")
}

func parseCacheFolder(folder string) (source, artist, title, leaf string) {
	folder = sanitizeCacheFolderPath(folder)
	if folder == "" {
		return "", "缓存音乐", "", ""
	}
	leaf = filepath.Base(filepath.FromSlash(folder))
	source = "cache"
	if strings.Contains(folder, "/") {
		parts := strings.Split(folder, "/")
		if len(parts) >= 2 && strings.TrimSpace(parts[0]) != "" {
			source = parts[0]
		}
	}
	artist = "缓存音乐"
	title = leaf
	if strings.Contains(leaf, "-") {
		parts := strings.SplitN(leaf, "-", 2)
		artist = strings.TrimSpace(parts[0])
		title = strings.TrimSpace(parts[1])
	}
	return source, artist, title, leaf
}

func listCacheFolders() []string {
	_ = os.MkdirAll(cacheMusicRoot, 0755)
	results := make([]string, 0, 64)
	seen := map[string]bool{}
	_ = filepath.WalkDir(cacheMusicRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		if d.Name() != "music.mp3" {
			return nil
		}
		dir := filepath.Dir(path)
		rel, err := filepath.Rel(cacheMusicRoot, dir)
		if err != nil {
			return nil
		}
		folder := sanitizeCacheFolderPath(filepath.ToSlash(rel))
		if folder == "" || seen[folder] {
			return nil
		}
		seen[folder] = true
		results = append(results, folder)
		return nil
	})
	sort.Strings(results)
	return results
}

func resolveCacheFolder(folder string) string {
	folder = sanitizeCacheFolderPath(folder)
	if folder == "" {
		return ""
	}
	if info, err := os.Stat(cacheFilePath(folder, "music.mp3")); err == nil && !info.IsDir() {
		return folder
	}
	legacy := sanitizeCacheSegment(filepath.Base(filepath.FromSlash(folder)))
	if legacy != "" {
		if info, err := os.Stat(cacheFilePath(legacy, "music.mp3")); err == nil && !info.IsDir() {
			return legacy
		}
	}
	return ""
}
