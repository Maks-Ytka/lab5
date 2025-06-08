package datastore

import (
	"os"
	"path/filepath"
)

func mergeSegments(outputPath string, segments []*segment) (*segment, error) {
	// Ключ → останнє значення
	merged := make(map[string]string)
	for _, seg := range segments {
		for key := range seg.index {
			val, err := seg.get(key)
			if err == nil {
				merged[key] = val
			}
		}
	}

	// Створюємо новий сегмент
	mergedPath := filepath.Join(filepath.Dir(outputPath), "merged-segment")
	f, err := os.Create(mergedPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	newSeg, err := newSegment(mergedPath)
	if err != nil {
		return nil, err
	}

	for key, val := range merged {
		_ = newSeg.put(entry{key, val})
	}
	return newSeg, nil
}
