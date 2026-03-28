package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type AsyncCacheTask struct {
	Key          string `json:"key"`
	Song         string `json:"song"`
	Singer       string `json:"singer"`
	Status       string `json:"status"`
	SourceType   string `json:"source_type,omitempty"`
	TargetFolder string `json:"target_folder,omitempty"`
	Error        string `json:"error,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	LastRequest  string `json:"last_request"`
	RequestCount int    `json:"request_count"`
}

var (
	asyncCacheMu    sync.Mutex
	asyncCacheTasks = map[string]*AsyncCacheTask{}
	asyncCacheQueue chan string
	asyncWorkerOnce sync.Once
)

const (
	asyncTaskDir            = "./files/cache/tasks"
	asyncPlaceholderMP3     = "./files/system/placeholder.mp3"
	asyncRecentFailureRetry = 60 * time.Second
)

func initAsyncCacheManager() {
	asyncWorkerOnce.Do(func() {
		_ = os.MkdirAll(asyncTaskDir, 0755)
		_ = os.MkdirAll(filepath.Dir(asyncPlaceholderMP3), 0755)
		ensurePlaceholderAudio()
		loadAsyncCacheTasks()
		asyncCacheQueue = make(chan string, 64)
		go asyncCacheWorker()
		resumeAsyncCacheTasks()
	})
}

func ensurePlaceholderAudio() {
	if info, err := os.Stat(asyncPlaceholderMP3); err == nil && !info.IsDir() && info.Size() > 512 {
		return
	}
	tempPath := asyncPlaceholderMP3 + ".tmp.mp3"
	_ = os.Remove(tempPath)
	cmd := exec.Command("ffmpeg",
		"-y",
		"-f", "lavfi",
		"-i", "anullsrc=r=24000:cl=mono",
		"-t", "1.2",
		"-ac", "1",
		"-ar", "24000",
		"-codec:a", "libmp3lame",
		"-b:a", "32k",
		"-write_xing", "0",
		"-id3v2_version", "0",
		tempPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("[AsyncCache] generate placeholder failed: %v %s\n", err, string(out))
		_ = os.WriteFile(asyncPlaceholderMP3, []byte("ID3"), 0644)
		return
	}
	if info, err := os.Stat(tempPath); err == nil && info.Size() > 512 {
		_ = os.Remove(asyncPlaceholderMP3)
		_ = os.Rename(tempPath, asyncPlaceholderMP3)
		return
	}
	_ = os.Remove(tempPath)
}

func asyncCacheTaskFile(key string) string {
	return filepath.Join(asyncTaskDir, key+".json")
}

func asyncCacheKey(song, singer string) string {
	raw := strings.TrimSpace(song) + "__" + strings.TrimSpace(singer)
	raw = toLower(strings.TrimSpace(raw))
	return sanitizeLocalFilename(raw)
}

func loadAsyncCacheTasks() {
	entries, err := os.ReadDir(asyncTaskDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(asyncTaskDir, entry.Name()))
		if err != nil {
			continue
		}
		var task AsyncCacheTask
		if err := json.Unmarshal(b, &task); err != nil {
			continue
		}
		if task.Key == "" {
			continue
		}
		if task.Status == "running" {
			task.Status = "queued"
			if strings.TrimSpace(task.Error) == "" {
				task.Error = "resumed_after_restart"
			}
		}
		asyncCacheTasks[task.Key] = &task
	}
}

func resumeAsyncCacheTasks() {
	for key, task := range asyncCacheTasks {
		if task == nil {
			continue
		}
		if task.Status != "queued" {
			continue
		}
		select {
		case asyncCacheQueue <- key:
			fmt.Printf("[AsyncCache] resumed task: %s (%s - %s)\n", key, task.Singer, task.Song)
		default:
			go func(k string) { asyncCacheQueue <- k }(key)
			fmt.Printf("[AsyncCache] resumed task in background: %s (%s - %s)\n", key, task.Singer, task.Song)
		}
	}
}

func saveAsyncCacheTask(task *AsyncCacheTask) {
	if task == nil || task.Key == "" {
		return
	}
	task.UpdatedAt = time.Now().Format(time.RFC3339)
	b, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(asyncCacheTaskFile(task.Key), b, 0644)
}

func asyncPlaceholderMusicItem(song, singer, status string) MusicItem {
	title := strings.TrimSpace(song)
	artist := strings.TrimSpace(singer)
	if title == "" {
		title = "正在准备音乐"
	}
	if artist == "" {
		artist = "远端流式首播"
	}
	streamURL := buildFirstPlayURL(song, singer)
	return MusicItem{
		Title:        title,
		Artist:       artist,
		Filename:     "firstplay.mp3",
		AudioURL:     streamURL,
		AudioFullURL: streamURL,
		Duration:     0,
		FromCache:    false,
		SourceType:   status,
	}
}

func readAsyncTaskTarget(task *AsyncCacheTask) (MusicItem, bool) {
	if task == nil {
		return MusicItem{}, false
	}
	folder := strings.TrimSpace(task.TargetFolder)
	if folder == "" {
		return MusicItem{}, false
	}
	if decoded, err := url.PathUnescape(folder); err == nil && strings.TrimSpace(decoded) != "" {
		folder = decoded
	}
	return buildCacheMusicItem(folder)
}

func enqueueAsyncCacheTask(song, singer string) MusicItem {
	initAsyncCacheManager()

	song = strings.TrimSpace(song)
	singer = strings.TrimSpace(singer)
	if song == "" {
		return MusicItem{}
	}

	if item, ok := findExactCacheMusic(song, singer); ok {
		return item
	}
	if item := getLocalMusicItem(song, singer); item.Title != "" {
		return item
	}

	key := asyncCacheKey(song, singer)
	now := time.Now()

	asyncCacheMu.Lock()
	task, ok := asyncCacheTasks[key]
	if !ok {
		task = &AsyncCacheTask{
			Key:          key,
			Song:         song,
			Singer:       singer,
			Status:       "queued",
			CreatedAt:    now.Format(time.RFC3339),
			UpdatedAt:    now.Format(time.RFC3339),
			LastRequest:  now.Format(time.RFC3339),
			RequestCount: 1,
		}
		asyncCacheTasks[key] = task
		saveAsyncCacheTask(task)
		select {
		case asyncCacheQueue <- key:
		default:
			go func(k string) { asyncCacheQueue <- k }(key)
		}
		asyncCacheMu.Unlock()
		fmt.Printf("[AsyncCache] queued new task: %s (%s - %s)\n", key, singer, song)
		return asyncPlaceholderMusicItem(song, singer, "async_pending")
	}

	task.LastRequest = now.Format(time.RFC3339)
	task.RequestCount++

	if task.Status == "done" {
		targetSong := task.Song
		targetSinger := task.Singer
		taskCopy := *task
		asyncCacheMu.Unlock()
		if item, ok := readAsyncTaskTarget(&taskCopy); ok {
			return item
		}
		if item, ok := findExactCacheMusic(targetSong, targetSinger); ok {
			return item
		}
		asyncCacheMu.Lock()
		task.Status = "queued"
		task.Error = "cache_target_missing"
		saveAsyncCacheTask(task)
		select {
		case asyncCacheQueue <- key:
		default:
			go func(k string) { asyncCacheQueue <- k }(key)
		}
		asyncCacheMu.Unlock()
		return asyncPlaceholderMusicItem(song, singer, "async_pending")
	}

	if task.Status == "failed" {
		if ts, err := time.Parse(time.RFC3339, task.UpdatedAt); err == nil && now.Sub(ts) >= asyncRecentFailureRetry {
			task.Status = "queued"
			task.Error = ""
			saveAsyncCacheTask(task)
			select {
			case asyncCacheQueue <- key:
			default:
				go func(k string) { asyncCacheQueue <- k }(key)
			}
		} else {
			saveAsyncCacheTask(task)
		}
		asyncCacheMu.Unlock()
		return asyncPlaceholderMusicItem(song, singer, "async_pending")
	}

	saveAsyncCacheTask(task)
	status := task.Status
	asyncCacheMu.Unlock()
	if status == "running" {
		return asyncPlaceholderMusicItem(song, singer, "async_running")
	}
	return asyncPlaceholderMusicItem(song, singer, "async_pending")
}

func asyncCacheWorker() {
	for key := range asyncCacheQueue {
		processAsyncCacheTask(key)
	}
}

func processAsyncCacheTask(key string) {
	asyncCacheMu.Lock()
	task, ok := asyncCacheTasks[key]
	if !ok {
		asyncCacheMu.Unlock()
		return
	}
	if task.Status == "running" {
		asyncCacheMu.Unlock()
		return
	}
	task.Status = "running"
	task.Error = ""
	saveAsyncCacheTask(task)
	song := task.Song
	singer := task.Singer
	asyncCacheMu.Unlock()

	fmt.Printf("[AsyncCache] start task: %s (%s - %s)\n", key, singer, song)
	item := requestAndCacheMusic(song, singer)

	asyncCacheMu.Lock()
	defer asyncCacheMu.Unlock()
	task, ok = asyncCacheTasks[key]
	if !ok {
		return
	}

	if item.Title != "" {
		task.Status = "done"
		task.SourceType = item.SourceType
		folder := strings.TrimPrefix(item.AudioURL, "/cache/music/")
		folder = strings.TrimSuffix(folder, "/music.mp3")
		folder = strings.TrimSpace(folder)
		if folder != "" {
			task.TargetFolder = folder
		} else if cachedItem, ok := findExactCacheMusic(song, singer); ok {
			folder = strings.TrimPrefix(cachedItem.AudioURL, "/cache/music/")
			folder = strings.TrimSuffix(folder, "/music.mp3")
			task.TargetFolder = strings.TrimSpace(folder)
		}
		saveAsyncCacheTask(task)
		fmt.Printf("[AsyncCache] task done: %s (%s)\n", key, task.TargetFolder)
		return
	}

	task.Status = "failed"
	task.Error = "legacy+youtube all missed"
	saveAsyncCacheTask(task)
	fmt.Printf("[AsyncCache] task failed: %s\n", key)
}

func HandleAsyncCacheTaskStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	initAsyncCacheManager()
	key := asyncCacheKey(r.URL.Query().Get("song"), r.URL.Query().Get("singer"))
	asyncCacheMu.Lock()
	defer asyncCacheMu.Unlock()
	if key != "" {
		if task, ok := asyncCacheTasks[key]; ok {
			_ = json.NewEncoder(w).Encode(task)
			return
		}
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}
	tasks := make([]*AsyncCacheTask, 0, len(asyncCacheTasks))
	for _, task := range asyncCacheTasks {
		t := *task
		tasks = append(tasks, &t)
	}
	_ = json.NewEncoder(w).Encode(tasks)
}
