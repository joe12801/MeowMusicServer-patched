package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type YuafengAPIFreeResponse struct {
	Data struct {
		Song      string `json:"song"`
		Singer    string `json:"singer"`
		Cover     string `json:"cover"`
		AlbumName string `json:"album_name"`
		Music     string `json:"music"`
		Lyric     string `json:"lyric"`
	} `json:"data"`
}

// 枫雨API response handler with multiple API fallback
func YuafengAPIResponseHandler(sources, song, singer string) MusicItem {
	fmt.Printf("[Info] Fetching music data for %s by %s\n", song, singer)

	apiHosts := []string{
		"https://api.yuafeng.cn",
		"https://api-v2.yuafeng.cn",
		"https://api.yaohud.cn",
	}

	var pathSuffix string
	switch sources {
	case "kuwo":
		pathSuffix = "/API/ly/kwmusic.php"
	case "netease":
		pathSuffix = "/API/ly/wymusic.php"
	case "migu":
		pathSuffix = "/API/ly/mgmusic.php"
	case "baidu":
		pathSuffix = "/API/ly/bdmusic.php"
	default:
		return MusicItem{}
	}

	var fallbackItem MusicItem

	for i, host := range apiHosts {
		fmt.Printf("[Info] Trying API %d/%d: %s\n", i+1, len(apiHosts), host)
		item := tryFetchFromAPI(host+pathSuffix, song, singer)
		if item.Title != "" {
			if item.LyricURL != "" {
				fmt.Printf("[Success] ✓ Found music WITH lyrics from %s\n", host)
				return item
			}
			if fallbackItem.Title == "" {
				fallbackItem = item
				fmt.Printf("[Info] ○ Got music WITHOUT lyrics from %s, saved as fallback, continuing...\n", host)
			} else {
				fmt.Printf("[Info] ○ Got music WITHOUT lyrics from %s, trying next API...\n", host)
			}
		} else {
			fmt.Printf("[Warning] × API %s failed, trying next...\n", host)
		}
	}

	if fallbackItem.Title != "" {
		fmt.Println("[Info] ▶ All 3 APIs tried - no lyrics found, returning music without lyrics")
		return fallbackItem
	}

	fmt.Println("[Error] ✗ All 3 APIs failed completely")
	return MusicItem{}
}

func tryFetchFromAPI(APIurl, song, singer string) MusicItem {
	resp, err := http.Get(APIurl + "?msg=" + song + "&n=1")
	if err != nil {
		fmt.Println("[Error] Error fetching the data from API:", err)
		return MusicItem{}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("[Error] Error reading the response body:", err)
		return MusicItem{}
	}

	bodyStr := string(body)
	if len(bodyStr) > 0 && bodyStr[0] == '<' {
		fmt.Println("[Warning] API returned HTML instead of JSON")
		os.WriteFile("debug_api_response.html", body, 0644)
		if strings.Contains(bodyStr, `"song"`) && strings.Contains(bodyStr, `"singer"`) {
			jsonStart := strings.Index(bodyStr, "{")
			jsonEnd := strings.LastIndex(bodyStr, "}")
			if jsonStart != -1 && jsonEnd != -1 && jsonEnd > jsonStart {
				jsonStr := bodyStr[jsonStart : jsonEnd+1]
				var response YuafengAPIFreeResponse
				err = json.Unmarshal([]byte(jsonStr), &response)
				if err == nil {
					body = []byte(jsonStr)
					goto parseSuccess
				}
			}
		}
		fmt.Println("[Error] Cannot parse HTML response - API may be unavailable")
		return MusicItem{}
	}

parseSuccess:
	var response YuafengAPIFreeResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		fmt.Println("[Error] Error parsing API response:", err)
		return MusicItem{}
	}

	folder := cacheFolderBySource("legacy", response.Data.Singer, response.Data.Song)
	dirName := cacheDirPath(folder)
	err = os.MkdirAll(dirName, 0755)
	if err != nil {
		fmt.Println("[Error] Error creating directory:", err)
		return MusicItem{}
	}

	if response.Data.Music == "" {
		fmt.Println("[Warning] Music URL is empty")
		return MusicItem{}
	}

	ext := filepath.Ext(response.Data.Cover)
	remoteURLFile := filepath.Join(dirName, "remote_url.txt")
	os.WriteFile(remoteURLFile, []byte(response.Data.Music), 0644)

	// 歌词与封面可异步
	go func() {
		lyricData := response.Data.Lyric
		if lyricData == "获取歌词失败" {
			fetchLyricFromYaohu(response.Data.Song, response.Data.Singer, dirName)
		} else if !strings.HasPrefix(lyricData, "http://") && !strings.HasPrefix(lyricData, "https://") {
			lines := strings.Split(lyricData, "\n")
			lyricFilePath := filepath.Join(dirName, "lyric.lrc")
			file, err := os.Create(lyricFilePath)
			if err == nil {
				timeTagRegex := regexp.MustCompile(`^\[(\d+(?:\.\d+)?)\]`)
				for _, line := range lines {
					match := timeTagRegex.FindStringSubmatch(line)
					if match != nil {
						timeInSeconds, _ := strconv.ParseFloat(match[1], 64)
						minutes := int(timeInSeconds / 60)
						seconds := int(timeInSeconds) % 60
						milliseconds := int((timeInSeconds-float64(seconds))*1000) / 100 % 100
						formattedTimeTag := fmt.Sprintf("[%02d:%02d.%02d]", minutes, seconds, milliseconds)
						line = timeTagRegex.ReplaceAllString(line, formattedTimeTag)
					}
					file.WriteString(line + "\r\n")
				}
				file.Close()
			}
		} else {
			downloadFile(filepath.Join(dirName, "lyric.lrc"), lyricData)
		}
	}()

	go func() {
		downloadFile(filepath.Join(dirName, "cover"+ext), response.Data.Cover)
	}()

	// 音频必须同步完成
	outputMp3 := filepath.Join(dirName, "music.mp3")
	err = streamConvertAudio(response.Data.Music, outputMp3)
	if err != nil {
		fmt.Println("[Error] Stream convert failed, fallback to download+compress:", err)
		musicExt, err2 := getMusicFileExtension(response.Data.Music)
		if err2 != nil {
			fmt.Println("[Warning] Cannot identify music format, using default .mp3:", err2)
			musicExt = ".mp3"
		}
		fullPath := filepath.Join(dirName, "music_full"+musicExt)
		err = downloadFile(fullPath, response.Data.Music)
		if err == nil {
			err = compressAndSegmentAudio(fullPath, dirName)
		}
		if err != nil {
			fmt.Println("[Error] Fallback audio processing failed:", err)
			return MusicItem{}
		}
	}

	fileInfo, statErr := os.Stat(outputMp3)
	if statErr != nil || fileInfo.Size() < 1024 {
		fmt.Println("[Error] music.mp3 not ready or too small")
		return MusicItem{}
	}

	basePath := cacheBaseURL(folder)
	return MusicItem{
		Title:        response.Data.Song,
		Artist:       response.Data.Singer,
		Filename:     response.Data.Singer + "-" + response.Data.Song + ".mp3",
		CoverURL:     basePath + "/cover" + ext,
		LyricURL:     basePath + "/lyric.lrc",
		AudioFullURL: basePath + "/music.mp3",
		AudioURL:     basePath + "/music.mp3",
		M3U8URL:      basePath + "/music.m3u8",
		Duration:     GetDuration(outputMp3),
		FromCache:    true,
		SourceType:   "legacy_api",
	}
}

// YaohuQQMusicResponse 妖狐QQ音乐API响应结构
type YaohuQQMusicResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Songname string `json:"songname"`
		Name     string `json:"name"`
		Picture  string `json:"picture"`
		Musicurl string `json:"musicurl"`
		Viplrc   string `json:"viplrc"`
	} `json:"data"`
}

type YaohuLyricResponse struct {
	Code int `json:"code"`
	Data struct {
		Lyric string `json:"lyric"`
	} `json:"data"`
}

func fetchLyricFromYaohu(songName, artistName, dirPath string) bool {
	apiKey := "bXO9eq1pomwR1cyVhzX"
	apiURL := "https://api.yaohud.cn/api/music/qq"
	requestURL := fmt.Sprintf("%s?key=%s&msg=%s&n=1&size=hq", apiURL, apiKey, url.QueryEscape(songName))
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(requestURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}
	var qqResp YaohuQQMusicResponse
	if err := json.Unmarshal(body, &qqResp); err != nil {
		return false
	}
	if qqResp.Code != 200 || qqResp.Data.Viplrc == "" {
		return false
	}
	resp2, err := client.Get(qqResp.Data.Viplrc)
	if err != nil {
		return false
	}
	defer resp2.Body.Close()
	body2, err := io.ReadAll(resp2.Body)
	if err != nil {
		return false
	}
	lyricText := string(body2)
	if lyricText == "" || len(lyricText) < 10 {
		return false
	}
	lyricFilePath := filepath.Join(dirPath, "lyric.lrc")
	file, err := os.Create(lyricFilePath)
	if err != nil {
		return false
	}
	defer file.Close()
	_, err = file.WriteString(lyricText)
	return err == nil
}

func getRemoteMusicURLOnly(song, singer string) string {
	fmt.Printf("[Info] Getting remote music URL for: %s - %s\n", singer, song)
	apiHosts := []string{"https://api.yuafeng.cn", "https://api-v2.yuafeng.cn"}
	sources := []string{"kuwo", "netease", "migu"}
	pathMap := map[string]string{
		"kuwo":    "/API/ly/kwmusic.php",
		"netease": "/API/ly/wymusic.php",
		"migu":    "/API/ly/mgmusic.php",
	}
	client := &http.Client{Timeout: 15 * time.Second}
	for _, host := range apiHosts {
		for _, source := range sources {
			path := pathMap[source]
			apiURL := fmt.Sprintf("%s%s?msg=%s-%s&n=1", host, path, url.QueryEscape(song), url.QueryEscape(singer))
			resp, err := client.Get(apiURL)
			if err != nil {
				continue
			}
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				continue
			}
			var response YuafengAPIFreeResponse
			if err := json.Unmarshal(body, &response); err != nil {
				continue
			}
			if response.Data.Music != "" {
				fmt.Printf("[Success] Got remote URL from %s: %s\n", source, response.Data.Music)
				return response.Data.Music
			}
		}
	}
	fmt.Println("[Error] Failed to get remote music URL from all APIs")
	return ""
}

func YuafengAPIResponseHandlerNoCache(sources, song, singer string) MusicItem {
	fmt.Printf("[Info] Fetching music data without cache for %s by %s\n", song, singer)

	apiHosts := []string{
		"https://api.yuafeng.cn",
		"https://api-v2.yuafeng.cn",
		"https://api.yaohud.cn",
	}

	var pathSuffix string
	switch sources {
	case "kuwo":
		pathSuffix = "/API/ly/kwmusic.php"
	case "netease":
		pathSuffix = "/API/ly/wymusic.php"
	case "migu":
		pathSuffix = "/API/ly/mgmusic.php"
	case "baidu":
		pathSuffix = "/API/ly/bdmusic.php"
	default:
		return MusicItem{}
	}

	for i, host := range apiHosts {
		fmt.Printf("[Info] Trying no-cache API %d/%d: %s\n", i+1, len(apiHosts), host)
		item := tryFetchFromAPINoCache(host+pathSuffix, song, singer, sources)
		if item.Title != "" {
			return item
		}
	}

	return MusicItem{}
}

func tryFetchFromAPINoCache(APIurl, song, singer, sourceType string) MusicItem {
	resp, err := http.Get(APIurl + "?msg=" + song + "&n=1")
	if err != nil {
		fmt.Println("[Error] Error fetching the data from API:", err)
		return MusicItem{}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("[Error] Error reading the response body:", err)
		return MusicItem{}
	}

	bodyStr := string(body)
	if len(bodyStr) > 0 && bodyStr[0] == '<' {
		fmt.Println("[Warning] API returned HTML instead of JSON")
		if strings.Contains(bodyStr, `"song"`) && strings.Contains(bodyStr, `"singer"`) {
			jsonStart := strings.Index(bodyStr, "{")
			jsonEnd := strings.LastIndex(bodyStr, "}")
			if jsonStart != -1 && jsonEnd != -1 && jsonEnd > jsonStart {
				jsonStr := bodyStr[jsonStart : jsonEnd+1]
				var response YuafengAPIFreeResponse
				if err := json.Unmarshal([]byte(jsonStr), &response); err == nil {
					body = []byte(jsonStr)
				}
			}
		}
	}

	var response YuafengAPIFreeResponse
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Println("[Error] Error parsing API response:", err)
		return MusicItem{}
	}
	if strings.TrimSpace(response.Data.Music) == "" {
		return MusicItem{}
	}

	lyricURL := ""
	if strings.HasPrefix(response.Data.Lyric, "http://") || strings.HasPrefix(response.Data.Lyric, "https://") {
		lyricURL = response.Data.Lyric
	}

	return MusicItem{
		Title:        response.Data.Song,
		Artist:       response.Data.Singer,
		Filename:     response.Data.Singer + "-" + response.Data.Song + ".mp3",
		AudioFullURL: response.Data.Music,
		AudioURL:     response.Data.Music,
		LyricURL:     lyricURL,
		CoverURL:     response.Data.Cover,
		Duration:     0,
		FromCache:    false,
		SourceType:   sourceType,
	}
}
