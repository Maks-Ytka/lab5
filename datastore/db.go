package datastore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxSegmentSize = 100
const outFileName = "current-data"

type Db struct {
	segments []*segment
	current  *segment
	dir      string
}

func Open(dir string) (*Db, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var segs []*segment

	for _, f := range files {
		if !f.Type().IsRegular() {
			continue
		}
		name := f.Name()
		if strings.HasPrefix(name, "segment-") || name == outFileName {
			s, err := newSegment(filepath.Join(dir, name))
			if err != nil {
				return nil, err
			}
			segs = append(segs, s)
		}
	}

	if len(segs) == 0 {
		s, err := newSegment(filepath.Join(dir, outFileName))
		if err != nil {
			return nil, err
		}
		segs = append(segs, s)
	}

	// Відсортувати сегменти: segment-0, segment-1, ..., current-data останнім
	sort.Slice(segs, func(i, j int) bool {
		return segs[i].path < segs[j].path
	})

	return &Db{
		segments: segs,
		current:  segs[len(segs)-1],
		dir:      dir,
	}, nil
}

func (db *Db) Close() error {
	for _, seg := range db.segments {
		_ = seg.close()
	}
	return nil
}

func (db *Db) Get(key string) (string, error) {
	for i := len(db.segments) - 1; i >= 0; i-- {
		val, err := db.segments[i].get(key)
		if err == nil {
			return val, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return "", err
		}
	}
	return "", ErrNotFound
}

func (db *Db) Put(key, value string) error {
	err := db.current.put(entry{key, value})
	if err != nil {
		return err
	}
	if db.current.size() >= maxSegmentSize {
		newPath := filepath.Join(db.dir, fmt.Sprintf("segment-%d", len(db.segments)))
		newSeg, err := newSegment(newPath)
		if err != nil {
			return err
		}
		db.segments = append(db.segments, newSeg)
		db.current = newSeg
	}
	return nil
}

func (db *Db) Size() (int64, error) {
	var total int64
	for _, seg := range db.segments {
		total += seg.size()
	}
	return total, nil
}
