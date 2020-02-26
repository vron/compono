package ztream

import (
	"archive/zip"
	"bytes"
	"encoding/hex"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
)

var dir string

func init() {
	var err error
	dir, err = ioutil.TempDir("", "zstream")
	if err != nil {
		panic(err.Error())
	}
}

var tOpt = Options{
	FileSize:             1024 * 1024,
	Verifier:             nil,
	SampleCompressSize:   1024,
	CompressionThreshold: 0.8,
}

type T interface {
	Error(a ...interface{})
	Log(a ...interface{})
	FailNow()
}

func file(t T) string {
	f, err := ioutil.TempFile(dir, "*.zip")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	name := f.Name()
	f.Close()
	err = os.Remove(name)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	t.Log("using file:", name)
	return name
}

func clean() {
	fis, _ := ioutil.ReadDir(dir)
	for _, f := range fis {
		os.Remove(f.Name())
	}
}

func data(le int, compressible bool) []byte {
	if !compressible {
		b := make([]byte, le)
		rand.Read(b)
		return b
	}
	// encoding to hex should allow plenty of compression ~50%
	r := make([]byte, (le)/2)
	rand.Read(r)
	b := make([]byte, le)
	hex.Encode(b, r)
	if le%2 != 0 {
		b[le-1] = 1
	}
	return b
}

func contains(t *testing.T, fn string, name string, data []byte) {
	r, err := zip.OpenReader(fn)
	if err != nil {
		t.Error(err)
		return
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name == name {
			rc, err := f.Open()
			defer rc.Close()
			if err != nil {
				t.Error(err)
			}
			buf, err := ioutil.ReadAll(rc)
			if err != nil {
				t.Error(err, len(buf))
			}
			if bytes.Equal(data, buf) {
				return
			}
			t.Error("decoded data not equal")
		}
	}
	t.Error("the file did not contain: ", name)
}

func validZip(t *testing.T, fn string, nofiles int) {
	// check with different parsers - ideally if we could find a string parser use that instead
	validZipGolang(t, fn, nofiles)
}

func validZipGolang(t *testing.T, fn string, nofiles int) {
	r, err := zip.OpenReader(fn)
	if err != nil {
		t.Error(err)
		return
	}
	defer r.Close()

	nof := 0
	for _, f := range r.File {
		nof++
		rc, err := f.Open()
		if err != nil {
			t.Error(err)
			t.FailNow()
			rc.Close()
		}
	}
	if nofiles >= 0 && nofiles != nof {
		t.Error("file did not contain ", nofiles, "files", nof)
	}
}
