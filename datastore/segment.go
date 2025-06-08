package datastore

import (
	"bufio"
	"errors"
	"io"
	"os"
)

type hashIndex map[string]int64

var ErrNotFound = errors.New("record does not exist")

type segment struct {
	file   *os.File
	index  hashIndex
	offset int64
	path   string
}

func newSegment(path string) (*segment, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	s := &segment{
		file:   f,
		path:   path,
		index:  make(hashIndex),
		offset: 0,
	}
	if err := s.rebuildIndex(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *segment) rebuildIndex() error {
	f, err := os.Open(s.path)
	if err != nil {
		return err
	}
	defer f.Close()

	in := bufio.NewReader(f)
	for {
		var rec entry
		n, err := rec.DecodeFromReader(in)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		s.index[rec.key] = s.offset
		s.offset += int64(n)
	}
	return nil
}

func (s *segment) put(e entry) error {
	n, err := s.file.Write(e.Encode())
	if err != nil {
		return err
	}
	s.index[e.key] = s.offset
	s.offset += int64(n)
	return nil
}

func (s *segment) get(key string) (string, error) {
	pos, ok := s.index[key]
	if !ok {
		return "", ErrNotFound
	}
	f, err := os.Open(s.path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	_, err = f.Seek(pos, 0)
	if err != nil {
		return "", err
	}
	var e entry
	if _, err = e.DecodeFromReader(bufio.NewReader(f)); err != nil {
		return "", err
	}
	return e.value, nil
}

func (s *segment) size() int64 {
	info, _ := s.file.Stat()
	return info.Size()
}

func (s *segment) close() error {
	return s.file.Close()
}
