package ztream

import (
	"bytes"
	"strconv"
	"testing"
)

func TestEmpty(t *testing.T) {
	fn := file(t)
	defer clean()

	s, _ := Create(fn, tOpt)
	if err := s.Close(); err != nil {
		t.Error(err)
	}
	validZip(t, fn, 0)
}

func TestAppend(t *testing.T) {
	fn := file(t)
	defer clean()

	s, _ := Create(fn, tOpt)
	d := data(12, false)
	_, err := s.Append("test", d)
	if err != nil {
		t.Error(err)
	}
	if err = s.Sync(); err != nil {
		t.Error(err)
	}
	s.Close()

	validZip(t, fn, 1)
	contains(t, fn, "test", d)
}

func TestAppendCompress(t *testing.T) {
	for _, size := range []int{500, 1024, 2000} {
		fn := file(t)
		defer clean()

		s, _ := Create(fn, tOpt)
		d := data(size, true)
		_, err := s.Append("test", d)
		if err != nil {
			t.Error(err)
		}
		if err = s.Sync(); err != nil {
			t.Error(err)
		}
		s.Close()

		validZip(t, fn, 1)
		contains(t, fn, "test", d)
	}
}

func TestEnsureCompress(t *testing.T) {
	fn := file(t)
	defer clean()

	s, _ := Create(fn, tOpt)
	d := data(int(tOpt.FileSize), true)
	_, err := s.Append("test", d)
	if err != nil {
		t.Error(err)
	}
	if err = s.Sync(); err != nil {
		t.Error(err)
	}
	s.Close()

	validZip(t, fn, 1)
	contains(t, fn, "test", d)
}

func TestMultiple(t *testing.T) {
	fn := file(t)
	defer clean()

	s, _ := Create(fn, tOpt)
	d1, d2, d3 := data(512, true), data(512, true), data(512, true)
	s.Append("test1", d1)
	s.Append("test2", d2)
	s.Append("test3", d3)
	s.Sync()
	s.Close()

	validZip(t, fn, 3)
	contains(t, fn, "test1", d1)
	contains(t, fn, "test2", d2)
	contains(t, fn, "test3", d3)
}

func TestRead(t *testing.T) {
	fn := file(t)
	defer clean()

	s, _ := Create(fn, tOpt)
	d1, d2, d3 := data(512, true), data(512, false), data(512, true)
	e1, _ := s.Append("test1", d1)
	e2, _ := s.Append("test2", d2)
	e3, _ := s.Append("test3", d3)
	s.Sync()

	buf := make([]byte, 512)
	err := s.Read(e1, buf)
	if !bytes.Equal(buf, d1) {
		t.Error(err, "not equal")
	}
	err = s.Read(e2, buf)
	if !bytes.Equal(buf, d2) {
		t.Error(err, "not equal")
	}
	err = s.Read(e3, buf)
	if !bytes.Equal(buf, d3) {
		t.Error(err, "not equal")
	}

	s.Close()

	validZip(t, fn, 3)
	contains(t, fn, "test1", d1)
	contains(t, fn, "test2", d2)
	contains(t, fn, "test3", d3)
}
func TestOpen(t *testing.T) {
	fn := file(t)
	defer clean()

	s, _ := Create(fn, tOpt)
	d1, d2, d3 := data(512, true), data(512, true), data(512, true)
	s.Append("test1", d1)
	s.Append("test2", d2)
	s.Append("test3", d3)
	c, err := s.Contents()
	if err != nil {
		t.Error(err)
	}
	if len(c) != 0 {
		t.Error("found contents when not expected", len(c))
	}
	s.Close()

	s, err = Open(fn, tOpt)
	if err != nil {
		t.Error(err)
	}

	c, err = s.Contents()
	if err != nil {
		t.Error(err)
	}
	if len(c) != 3 {
		t.Error("expected 3 contents")
	}

	d := data(512, false)
	s.Append("test4", d)
	s.Sync()

	s.Close()

	validZip(t, fn, 4)
	contains(t, fn, "test1", d1)
	contains(t, fn, "test2", d2)
	contains(t, fn, "test3", d3)
	contains(t, fn, "test4", d)
}

func TestOpenVerify(t *testing.T) {
	fn := file(t)
	defer clean()

	s, _ := Create(fn, tOpt)
	d1, d2, d3 := data(512, true), data(512, true), data(512, true)
	s.Append("test1", d1)
	s.Append("test2", d2)
	s.Append("test3", d3)
	c, err := s.Contents()
	if err != nil {
		t.Error(err)
	}
	if len(c) != 0 {
		t.Error("found bcontents when not expected", len(c))
	}
	s.Close()

	o := tOpt
	var v ver
	o.Verifier = &v
	s, err = Open(fn, o)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	s.Close()

	validZip(t, fn, 3)
	contains(t, fn, "test1", d1)
	contains(t, fn, "test2", d2)
	contains(t, fn, "test3", d3)
}

func TestOpenWipe(t *testing.T) {
	fn := file(t)
	defer clean()

	s, _ := Create(fn, tOpt)
	d1, d2, d3 := data(512, true), data(512, true), data(512, true)
	s.Append("test1", d1)
	s.Append("test2", d2)
	s.Append("test3", d3)
	s.Sync()
	if c, _ := s.Contents(); len(c) != 3 {
		t.Error("expected 3")
	}
	err := s.Wipe("test2")
	if err != nil {
		t.Error(err)
	}
	if c, _ := s.Contents(); len(c) != 2 {
		t.Error("expected 2")
	}
	s.Close()
	validZip(t, fn, 2)
	contains(t, fn, "test1", d1)
	contains(t, fn, "test3", d3)

	s, _ = Open(fn, tOpt)
	if c, _ := s.Contents(); len(c) != 2 {
		t.Error("expected 2, on open")
	}
	s.Close()

	validZip(t, fn, 2)
	contains(t, fn, "test1", d1)
	contains(t, fn, "test3", d3)
}

type ver int

func (v *ver) Write(b []byte) (int, error) {
	return len(b), nil
}

func (v *ver) Reset() {
	(*v)++
}

func (v *ver) Match(name string) bool {
	return name == "test"+strconv.Itoa(int(*v))
}
