package covers

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	_ "image/png"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dhowden/tag"
	"github.com/spf13/cobra"
)

var audioExts = map[string]bool{
	".mp3": true, ".flac": true, ".ogg": true, ".m4a": true,
}

func Run(cmd *cobra.Command, root string) error {
	threads, _ := cmd.Flags().GetInt("threads")
	force, _ := cmd.Flags().GetBool("replace")

	var dirs []string
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	})
	if walkErr != nil {
		return walkErr
	}

	jobs := make(chan string)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for dir := range jobs {
				if err := processDir(dir, force); err != nil {
					mu.Lock()
					errs = append(errs, err)
					mu.Unlock()
				}
			}
		}()
	}

	for _, d := range dirs {
		jobs <- d
	}
	close(jobs)
	wg.Wait()

	return errors.Join(errs...)
}

func processDir(dir string, force bool) error {
	coverPath := filepath.Join(dir, "cover.jpg")
	if !force {
		if info, err := os.Stat(coverPath); err == nil && info.Size() > 0 {
			return nil
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var lastErr error
	for _, e := range entries {
		if e.IsDir() || !audioExts[strings.ToLower(filepath.Ext(e.Name()))] {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := extractCover(path)
		if err != nil {
			lastErr = err
			continue
		}
		if data == nil {
			continue // no art in this file, try the next one
		}
		if err := writeAtomic(coverPath, data); err != nil {
			return err
		}
		log.Printf("wrote %s", coverPath)
		return nil
	}

	if lastErr != nil {
		return lastErr
	}
	return nil
}

func extractCover(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		return nil, err
	}
	pic := m.Picture()
	if pic == nil {
		return nil, nil
	}

	if pic.Ext == "jpg" || pic.Ext == "jpeg" {
		return pic.Data, nil
	}

	img, _, err := image.Decode(bytes.NewReader(pic.Data))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeAtomic(dst string, data []byte) error {
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}
