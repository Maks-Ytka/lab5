package datastore

import (
	"testing"
)

func TestSegmentMerge(t *testing.T) {
	tmp := t.TempDir()
	db, err := Open(tmp)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	const longValue = "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	for i := 0; i < 200; i++ {
		key := "k" + string(rune(i%10))
		err := db.Put(key, longValue+string(rune(i)))
		if err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	}

	if len(db.segments) <= 1 {
		t.Fatal("Expected multiple segments after many puts")
	}

	mergedSeg, err := mergeSegments(tmp, db.segments)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}
	t.Cleanup(func() {
		_ = mergedSeg.close()
	})

	for i := 0; i < 10; i++ {
		key := "k" + string(rune(i))
		val, err := mergedSeg.get(key)
		if err != nil {
			t.Errorf("Get from merged failed for key %s: %v", key, err)
		}
		if val == "" {
			t.Errorf("Merged value for %s is empty", key)
		}
	}
}
