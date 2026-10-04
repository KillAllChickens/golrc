package lrcgen

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/wtolson/go-taglib"
	"resty.dev/v3"
)

var errNotFound = errors.New("no lyrics found")

type lrclibSearchResult struct {
	lrclibResp
	Duration float64 `json:"duration"`
}

type lrclibResp struct {
	Name         string `json:"name"`
	PlainLyrics  string `json:"plainLyrics"`
	SyncedLyrics string `json:"syncedLyrics"`
}

func swapExt(path, ext string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + ext
}

func sanitizeName(s string) string {
	var badChars = strings.NewReplacer(
		"/", "-", "\\", "-", ":", "-", "*", "", "?", "",
		"\"", "'", "<", "", ">", "", "|", "-",
	)
	return strings.TrimSpace(badChars.Replace(s))
}


func processFile(ctx context.Context, path string, rename bool) (string, error) {
	f, err := taglib.Read(path)
	if err != nil {
		return path, err
	}
	track, title, artist, album := f.Track(), f.Title(), f.Artist(), f.Album()
	duration := int(f.Length().Seconds())
	f.Close()

	if rename && title != "" {
		name := sanitizeName(title)
		if track > 0 {
			name = fmt.Sprintf("%02d - %s", track, name)
		}
		newPath := filepath.Join(filepath.Dir(path), name+filepath.Ext(path))
		if newPath != path {
			if err := os.Rename(path, newPath); err != nil {
				return path, err
			}
			path = newPath
		}
	}

	lrcPath := strings.TrimSuffix(path, filepath.Ext(path)) + ".lrc"

	if info, err := os.Stat(lrcPath); err == nil && info.Size() > 0 {
		return path, nil
	}



	// fmt.Println("Title:   ", f.Title())
	// fmt.Println("Artist:  ", f.Artist())
	// fmt.Println("Length:  ", strconv.Itoa(int(f.Length().Seconds())))
	// fmt.Println("Bitrate: ", f.Bitrate())
	// fmt.Println("Samplerate:", f.Samplerate())
	//

	var lrcResp lrclibResp

	client := resty.New().
		SetHeader("User-Agent", "GOLRC v0.0.1 (https://github.com/KillAllChickens/golrc)").
		SetContext(ctx)


	lrcResp, err = fetchLyrics(client, artist, title, album, duration)
	if hasNoLyrics(lrcResp, err) && album != "" {
		// log.Printf("no usable lyrics with album %q for %s, retrying without album", f.Album(), path)
		lrcResp, err = fetchLyrics(client, artist, title, "", duration)
	}
	if hasNoLyrics(lrcResp, err) {
		// log.Printf("falling back to search for %s", path)
		lrcResp, err = searchLyrics(client, artist, title, duration)
	}
	if err != nil {
		return path, err
	}

	lyrics := lrcResp.SyncedLyrics
	if lyrics == "" {
		lyrics = lrcResp.PlainLyrics
	}
	if lyrics == "" {
		// log.Printf("lrclib matched %s but both plain and synced lyrics were empty", path)
		return path, nil
	}

	tmp := lrcPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(lyrics), 0o644); err != nil {
		return path, fmt.Errorf("writing lrc for %s: %w", path, err)
	}
	if err := os.Rename(tmp, lrcPath); err != nil {
		return path, fmt.Errorf("renaming lrc for %s: %w", path, err)
	}
	// log.Printf("wrote %s", lrcPath)

	return path, nil
}

func hasNoLyrics(r lrclibResp, err error) bool {
	return errors.Is(err, errNotFound) || (err == nil && r.PlainLyrics == "" && r.SyncedLyrics == "")
}

func searchLyrics(client *resty.Client, artist, track string, duration int) (lrclibResp, error) {
	var results []lrclibSearchResult

	resp, err := client.R().
		SetQueryParams(map[string]string{
			"artist_name": artist,
			"track_name":  track,
		}).
		SetResult(&results).
		Get("https://lrclib.net/api/search")
	if err != nil {
		return lrclibResp{}, err
	}
	if !resp.IsStatusSuccess() {
		return lrclibResp{}, fmt.Errorf("bad status %d", resp.StatusCode())
	}

	for _, r := range results {
		if abs(int(r.Duration)-duration) <= 2 && (r.PlainLyrics != "" || r.SyncedLyrics != "") {
			return r.lrclibResp, nil
		}
	}
	return lrclibResp{}, errNotFound
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func fetchLyrics(client *resty.Client, artist, track, album string, duration int) (lrclibResp, error) {
	var lrcResp lrclibResp

	// log.Printf("querying: artist=%q track=%q album=%q duration=%d", artist, track, album, duration)

	params := map[string]string{
		"artist_name": artist,
		"track_name":  track,
		"duration":    strconv.Itoa(duration),
	}
	if album != "" {
		params["album_name"] = album
	}

	resp, err := client.R().
		SetQueryParams(params).
		SetResult(&lrcResp).
		Get("https://lrclib.net/api/get")
	if err != nil {
		return lrcResp, err
	}
	if !resp.IsStatusSuccess() {
		// log.Printf("lrclib response for artist=%q track=%q album=%q: status=%d body=%s",
		// 	artist, track, album, resp.StatusCode(), resp.String())
		if resp.StatusCode() == 404 {
			return lrcResp, errNotFound
		}
		return lrcResp, fmt.Errorf("bad status %d", resp.StatusCode())
	}
	return lrcResp, nil
}

func processWithRetry(ctx context.Context, path string, maxAttempts int, rename bool) error {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		newPath, err := processFile(ctx, path, rename)
		path = newPath

		if err == nil {
			return nil
		}
		if errors.Is(err, errNotFound) {
			log.Printf("no lyrics for %s, skipping", path)
			return nil
		}

		lastErr = err
		log.Printf("failed %s (attempt %d/%d): %v", path, attempt, maxAttempts, err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
			continue
		}
	}

	return fmt.Errorf("giving up on %s after %d attempts: %w", path, maxAttempts, lastErr)
}

func Run(cmd *cobra.Command, root string) error {
	concurrency, _ := cmd.Flags().GetInt("threads")
	doRename, _ := cmd.Flags().GetBool("rename")


	ctx := context.Background()

	audioExts := map[string]bool{
		".mp3": true, ".wav": true, ".flac": true, ".ogg": true, ".m4a": true, ".aac": true,
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !audioExts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		// log.Printf("Finding lyrics for %s...", path)

		sem <- struct{}{}
		wg.Add(1)

		go func(p string) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := processWithRetry(ctx, p, 10, doRename); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(path)

		return nil
	})

	wg.Wait()

	if walkErr != nil {
		return walkErr
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
