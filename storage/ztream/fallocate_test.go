package ztream

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestFallocate(t *testing.T) {
	defer clean()

	file := filepath.Join(dir, "fallocate.zip")
	t.Log(file)

	s, err := Create(file, Options{})
	if err != nil {
		t.Error(err)
	}
	defer s.Close()

	// Ensure that the size of the file is now what is expected
	fi, err := os.Stat(file)
	if err != nil {
		t.Error(err)
	}

	if fi.Size() != int64(DefaultOptions.FileSize) {
		t.Error("the file has not been fallocated to the expected size", fi.Size(), int64(DefaultOptions.FileSize))
	}

	// Also ensure that all bytes will be zero - since our file scanning will depend on that!
	// also double check the size we read
	buf := make([]byte, 1024*1024)
	nor := 0
	f, err := os.Open(file)
	if err != nil {
		t.Error(err)
	}
	defer f.Close()
	for {
		n, err := f.Read(buf)
		nor += n
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
		for _, v := range buf {
			if v != 0 {
				t.Error("the fallocated file was not all zero")
				t.FailNow()
			}
		}
	}

	if nor != int(DefaultOptions.FileSize) {
		t.Error("size of read file does not match")
	}
}
