package cover

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dhowden/tag"
)

var folderNames = []string{"cover", "folder", "front", "albumart", "album", "artwork", "art"}

var imageExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true, ".bmp": true}

func FromFile(path string) []byte {
	if data := embedded(path); data != nil {
		return data
	}
	return fromFolder(filepath.Dir(path))
}

func embedded(path string) (data []byte) {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	defer func() {
		if recover() != nil {
			data = nil
		}
	}()
	m, err := tag.ReadFrom(f)
	if err != nil {
		return nil
	}
	p := m.Picture()
	if p == nil || len(p.Data) > MaxSize || Sniff(p.Data) == "" {
		return nil
	}
	return p.Data
}

func fromFolder(dir string) []byte {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	type cand struct {
		name string
		rank int
	}
	var list []cand
	images := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if !imageExts[ext] {
			continue
		}
		images++
		stem := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
		rank := -1
		for i, want := range folderNames {
			if stem == want {
				rank = i
				break
			}
		}
		switch {
		case rank >= 0:
		case strings.HasPrefix(stem, "albumart") && strings.Contains(stem, "large"):
			rank = len(folderNames)
		case strings.HasPrefix(stem, "albumart"):
			rank = len(folderNames) + 1
		case strings.Contains(stem, "cover") || strings.Contains(stem, "front"):
			rank = len(folderNames) + 2
		default:
			rank = len(folderNames) + 3
		}
		list = append(list, cand{name, rank})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].rank != list[j].rank {
			return list[i].rank < list[j].rank
		}
		return list[i].name < list[j].name
	})
	for _, c := range list {
		if c.rank == len(folderNames)+3 && images != 1 {
			break
		}
		if data := readImage(filepath.Join(dir, c.name)); data != nil {
			return data
		}
	}
	return nil
}
