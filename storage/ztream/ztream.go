// Package ztream implements zip files for streaming data to.
package ztream

import (
	"bufio"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"hash"
	"hash/crc32"
	"io"
	"math"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/detailyang/go-fallocate"
)

var (
	ErrStreamFull        = errors.New("zstream: the provided data does not fit in the stream")
	ErrBuffNotSufficient = errors.New("zstream: the provided buffer is not long enough to read the data")
)

// TODO: Minimize garbage
// TODO: change to using int32 - we will never need larger files...
// TODO: document that only intended for small files that can be kept fully in memory

// TODO: Expose statistics, e.g compressed etc.

// TODO: Keep errors from Append to return at sync

const (
	bufferSize = 1024 * 16
	// maxNameLength must never be decreased to maintain compatibility
	maxNameLength = bufferSize - 46
)

// A Stream can only b
type Stream struct {
	m sync.RWMutex // protects all fields

	opt          Options
	file         *os.File // the underlying file on disk to write to
	compressor   *flate.Writer
	decompressor io.ReadCloser
	entries      []entry // entries that are synced to disk
	pending      []entry // entries appended and written but not yet synced to disk

	loaded     bool // true if the file has been loaded/parsed
	lastAppend bool // if the file handler is seeked so we can just append

	reader   *bufio.Reader
	buffer   []byte
	lastRead int
	crc      hash.Hash32
}

// Create h
func Create(path string, opt Options) (s *Stream, err error) {
	if err = getOptions(&opt); err != nil {
		return nil, err
	}

	s = &Stream{opt: opt, pending: make([]entry, 0, 32), reader: bufio.NewReader(nil), buffer: make([]byte, bufferSize), decompressor: flate.NewReader(nil)}
	s.file, err = os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0777)
	if err != nil {
		return nil, err
	}
	err = fallocate.Fallocate(s.file, 0, int64(opt.FileSize))
	if err != nil {
		s.file.Close()
		return nil, err
	}

	return s, s.load()
}

// Open op
func Open(path string, opt Options) (s *Stream, err error) {
	if err = getOptions(&opt); err != nil {
		return nil, err
	}

	// The file size we need to get from the file
	// TODO: Can this be delayed to avoid a disk seek for Stat if we only want to read?
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	s = &Stream{opt: opt, reader: bufio.NewReader(nil), buffer: make([]byte, bufferSize), lastRead: -2, decompressor: flate.NewReader(nil)}

	size := fi.Size()
	if size > math.MaxInt32 {
		return nil, s.corruptError(0, "file is to large to be a ztream")
	}
	s.opt.FileSize = int32(size)

	s.file, err = os.OpenFile(path, os.O_RDWR, 0777)
	if err != nil {
		return nil, err
	}

	if s.opt.Verifier != nil {
		err := s.load()
		if err != nil {
			return nil, err
		}
	}

	return s, nil
}

// Close ensures that the written file is a valid zip and everything
// is commited to disk.
func (s *Stream) Close() error {
	s.m.Lock()
	defer s.m.Unlock()

	if err := s.sync(); err != nil {
		return err
	}

	if !s.loaded {
		// it has been opened in read-only mode - no need to write anything
		return s.file.Close()
	}

	// Write out everything, including the end of file dictionary
	err := s.writeDirectory(0)
	if err != nil {
		_ = s.file.Close()
		return err
	}
	return s.file.Close()
}

type Entry struct {
	Name   string
	Offset int32 // Offset to the actuall data, not the header.
	// The data is deflated if and only if CompressedSize < UncompressedSize
	CompressedSize   int32
	UncompressedSize int32
}

type entry struct {
	Entry
	crc     uint32
	modTime uint16
	modDate uint16
}

// Contents returns a list of the file names contained in the ztream,
// in the same order as stored on disk. Note that data appended but not Synced
// will not be returned here.
func (s *Stream) Contents() (entries []Entry, err error) {
	s.m.Lock()
	defer s.m.Unlock()

	if !s.loaded {
		err = s.load()
	}
	if err == nil {
		entries = make([]Entry, len(s.entries))
		for i := range entries {
			entries[i] = s.entries[i].Entry
		}
	}
	return
}

// the caller is expected to hold a write lock and it to be loaded
func (s *Stream) enoughSpace(name string, data []byte) bool {
	available := int(s.opt.FileSize)
	names := 0
	if len(s.pending) > 0 {
		e := s.pending[len(s.pending)-1]
		available -= int(e.Offset) + int(e.CompressedSize)
		for _, p := range s.pending {
			names += len(p.Name)
		}
	} else if len(s.entries) > 0 {
		e := s.entries[len(s.entries)-1]
		available -= int(e.Offset) + int(e.CompressedSize)
		for _, e := range s.entries {
			names += len(e.Name)
		}
	}
	noFiles := len(s.entries) + len(s.pending)
	eofSize := 46*noFiles + 24 + names
	available -= eofSize

	available -= 46 + len(name)*2 + 30 + len(data)

	return available > 0
}

// Append tries to append a file with the given name and data to the file.
// NOTE that this will NOT commit the data to disk, Sync() MUST be called, this
// allows seceral data pieces to be written at once to avoid disk seek.
// If it does not fit ErrStreamFull is returned.
func (s *Stream) Append(name string, data []byte) (Entry, error) {
	s.m.Lock()
	defer s.m.Unlock()
	s.lastRead = -2

	if !s.loaded {
		if err := s.load(); err != nil {
			return Entry{}, err
		}
	}

	// figure out if we should compress or not.
	doCompress := false
	if s.opt.SampleCompressSize > 0 {
		var testCompression []byte
		if len(data) <= s.opt.SampleCompressSize {
			testCompression = data
		} else {
			testCompression = data[:s.opt.SampleCompressSize]
		}
		cw := countWriter{}
		s.compressor.Reset(&cw)
		s.compressor.Write(testCompression)
		s.compressor.Close()
		if cw.Size() > 0 && cw.Size() < int(s.opt.CompressionThreshold*float32(len(testCompression))) {
			doCompress = true
		}
	}

	var buff []byte = data
	if doCompress {
		// choosing to allocate each time instead of retaining since we assume
		// the gc cost overhead is relatively small compared to the disk operations we do here anyway.
		bw := bytes.NewBuffer(make([]byte, 0, len(data)))
		s.compressor.Reset(bw)
		n, err := s.compressor.Write(data)
		if err != nil {
			return Entry{}, err
		}
		if n != len(data) {
			panic("should not be able to happen?")
		}
		if err := s.compressor.Close(); err != nil {
			return Entry{}, err
		}
		buff = bw.Bytes()
		if len(buff) >= len(data) {
			// if compression made it worse - discard it..
			buff = data
			doCompress = false
		}
	}

	if !s.enoughSpace(name, buff) {
		return Entry{}, ErrStreamFull
	}

	// ensure the file is ready to be written
	offset := int32(0)
	if len(s.pending) > 0 {
		e := s.pending[len(s.pending)-1]
		offset = e.Offset + e.CompressedSize
	} else if len(s.entries) > 0 {
		e := s.entries[len(s.entries)-1]
		offset = int32(e.Offset + e.CompressedSize)
	}
	if !s.lastAppend {
		s.file.Seek(int64(offset), 0)
	}

	// calculate the crc
	crc := crc32.NewIEEE()
	crc.Write(data)

	// we now have the data we should write in buff, but we first need
	// to write the header.
	// TODO: should we retain this buffer instead of allocating new?
	header := make([]byte, 30+len(name))
	header, time, date := encodeFileHeader(header, false, crc.Sum32(), int32(len(buff)), int32(len(data)), name)

	if _, err := s.file.Write(header); err != nil {
		return Entry{}, err
	}
	if _, err := s.file.Write(buff); err != nil {
		return Entry{}, err
	}

	s.lastAppend = true
	ee := Entry{Name: name,
		Offset:           offset + 30 + int32(len(name)),
		UncompressedSize: int32(len(data)),
		CompressedSize:   int32(len(buff))}
	s.pending = append(s.pending, entry{
		Entry:   ee,
		crc:     crc.Sum32(),
		modTime: time,
		modDate: date,
	})
	return ee, nil
}

// Sync flushes out all the Appended data to the underlying disk (writes and syncs).
// Note that the end of file dictionary will not be written until Close is called, since data can be recovered by scanning.
// Note that after an error here 0 or more of the data pieces Appended since last Sync
// may be missing from the file - they must be read and checked to ensure they are
// present.
func (s *Stream) Sync() error {
	s.m.Lock()
	defer s.m.Unlock()
	if !s.loaded {
		return nil // nothing to do
	}

	return s.sync()
}

func (s *Stream) sync() error {
	if len(s.pending) <= 0 {
		return nil
	}

	err := s.file.Sync()
	s.entries = append(s.entries, s.pending...)
	s.pending = s.pending[:0]
	return err
}

// Read out the Entry, optimized for sequential reading of entries after
// each other. Note that buf must be large enough to hold the uncompressed
// data or it is an error of type ErrBuffNotSufficient.
func (s *Stream) Read(e Entry, buf []byte) (err error) {
	s.m.RLock()
	defer s.m.RUnlock()
	s.lastAppend = false

	offsetToStart := int(e.Offset) - 30 - len(e.Name)
	if s.lastRead != offsetToStart-1 {
		// slow-path for now-sequential reads
		_, err := s.file.Seek(int64(offsetToStart), 0)
		if err != nil {
			s.lastRead = -2
			return err
		}
		s.reader.Reset(s.file)
	}
	s.lastRead = -2

	// thanks to the entry we know the header size we need read out
	lfh, err := s.decodeFileHeader(offsetToStart, s.buffer, s.reader)
	if err != nil {
		return err
	}
	if lfh == nil {
		return errors.New("zstream: did not find a file at specified offset")
	}

	if lfh.fileName != e.Name {
		return errors.New("zstream: name not matching")
	}
	if lfh.compressedSize != e.CompressedSize {
		return errors.New("zstream: compressed size not matchin")
	}
	if lfh.uncompressedSize != e.UncompressedSize {
		return errors.New("zstream: compressed size not matchin")
	}
	if len(buf) < int(lfh.uncompressedSize) {
		return ErrBuffNotSufficient
	}

	// check the crc and afterwards deflate if needed
	if s.crc == nil {
		s.crc = crc32.NewIEEE()
	}
	crc := s.crc
	crc.Reset()
	if !lfh.deflated() {
		_, err := io.ReadFull(s.reader, buf[:lfh.uncompressedSize])
		if err != nil {
			return err
		}
		crc.Write(buf[:lfh.uncompressedSize])
		if crc.Sum32() != lfh.cRC {
			return s.corruptError(offsetToStart, "stored crc code not matching, indicating corrupted data")
		}
		s.lastRead = int(e.Offset + e.CompressedSize)
		return nil
	}

	err = s.decompressor.(flate.Resetter).Reset(s.reader, nil)
	if err != nil {
		return err
	}
	n, err := io.ReadFull(s.decompressor, buf[:lfh.uncompressedSize])
	if n != int(lfh.uncompressedSize) {
		if err != nil {
			return err
		}
		return errors.New("zstream: decompressed data has wrong size")
	}

	crc.Write(buf[:lfh.uncompressedSize])
	if crc.Sum32() != lfh.cRC {
		return s.corruptError(offsetToStart, "stored crc code not matching, indicating corrupted data")
	}
	s.lastRead = int(e.Offset + e.CompressedSize)
	return nil
}

// Wipe removes the given file and overwrites the data twice to ensure that it is
// gone. Note that this is an expensive call. It is not expected to be used often so ok that it is slow.
func (s *Stream) Wipe(name string) error {
	s.m.Lock()
	defer s.m.Unlock()
	s.lastRead = -2
	s.lastAppend = false

	if !s.loaded {
		if err := s.load(); err != nil {
			return err
		}
	} else {
		s.sync()
	}

	// entries are not sorted so we must do a linear scan.
	e := entry{Entry: Entry{Offset: -1}}
	ei := -1
	for i := range s.entries {
		if s.entries[i].Name == name {
			e = s.entries[i]
			ei = i
		}
	}
	if e.Offset < 0 {
		// since we synced we know there is nothing pending
		return errors.New("zstream: no such entry")
	}

	// First write random data, after ensuring this is a file
	offsetToStart := int(e.Offset) - 30 - len(e.Name)
	if _, err := s.file.Seek(int64(offsetToStart), 0); err != nil {
		return err
	}
	s.reader.Reset(s.file)
	lfh, err := s.decodeFileHeader(offsetToStart, s.buffer, s.reader)
	if err != nil {
		return err
	}
	if lfh == nil {
		return s.corruptError(offsetToStart, "zstream: not a lfh where expected")
	}

	// Write put random data first
	dataSize := 30 + len(lfh.fileName) + int(lfh.compressedSize)
	if _, err := s.file.Seek(int64(offsetToStart), 0); err != nil {
		return err
	}
	n, err := io.CopyBuffer(s.file, io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), int64(dataSize)), s.buffer)
	if err != nil {
		// TODO: This error is bad - we should try to recover here since otherwise we might create an all corrupt file that is hard to recover
		// from.
		return err
	}
	if int(n) != dataSize {
		// TODO: This error is bad - we should try to recover here since otherwise we might create an all corrupt file that is hard to recover
		// from.
		return errors.New("zstream: did not manage to write the random data")
	}

	// Now we need to write out zero data, and also calculate the correct crc
	// code such that we still have a valid zip for a recover program.
	crc := crc32.NewIEEE()
	if _, err := s.file.Seek(int64(offsetToStart+30), 0); err != nil {
		return err
	}
	for i := range s.buffer {
		s.buffer[i] = 0
	}
	var b []byte
	for toWrite := dataSize - 30; toWrite > 0; toWrite -= len(b) {
		if toWrite > len(s.buffer) {
			b = s.buffer
		} else {
			b = s.buffer[:toWrite]
		}
		_, err := s.file.Write(b)
		if err != nil {
			return err
		}
		crc.Write(b)
	}

	b, _, _ = encodeFileHeader(s.buffer, true, crc.Sum32(), int32(dataSize)-30, int32(dataSize)-30, "")
	_, err = s.file.Seek(int64(offsetToStart), 0)
	if err != nil {
		return err
	}
	_, err = s.file.Write(b)
	if err != nil {
		return err
	}

	s.entries = append(s.entries[:ei], s.entries[ei+1:]...)
	return s.writeDirectory(len(lfh.fileName))
}

// load opens the file for writing and or verification. The caller is expected to hold
// a write lock. Note that we are scanning the file instead of using a directory - since
// we need to be able to open files that were not properly closed.
func (s *Stream) load() (err error) {
	s.compressor, err = flate.NewWriter(nil, s.opt.CompressionLevel)
	s.lastAppend = false
	s.lastRead = -2

	buf := make([]byte, bufferSize) // This should be cached in the struct?
	// TODO: if we have a verifier allocate a larger buffer since we will need to read the entire data stream
	reader := s.reader
	offset := 0
	reader.Reset(s.file)

	for {
		offs := offset
		lfh, err := s.decodeFileHeader(offset, buf, reader)
		if err != nil {
			return err
		}
		if lfh == nil {
			break // no more files to find
		}

		if lfh.wiped() {
			offset += 30 + len(lfh.fileName) + int(lfh.compressedSize)
			if _, err := s.file.Seek(int64(offset), 0); err != nil {
				return err
			}
			reader.Reset(s.file)
			continue
		}

		if s.opt.Verifier != nil {
			if err := s.verifyData(offset, buf, lfh, reader); err != nil {
				return errors.New("sss" + err.Error())
			}
			offset += 30 + len(lfh.fileName) + int(lfh.compressedSize)
		} else {
			offset += 30 + len(lfh.fileName) + int(lfh.compressedSize)
			if _, err := s.file.Seek(int64(offset), 0); err != nil {
				return err
			}
			reader.Reset(s.file)

		}

		s.entries = append(s.entries, entry{Entry: Entry{
			Name:             lfh.fileName,
			Offset:           int32(offs) + 30 + int32(len(lfh.fileName)),
			CompressedSize:   lfh.compressedSize,
			UncompressedSize: lfh.uncompressedSize,
		},
			crc:     lfh.cRC,
			modTime: lfh.modificationTime,
			modDate: lfh.modificationDate,
		})
	}

	s.loaded = true
	return nil
}

// verify both that the crc code and that the verifier gives the correct values,
// this requires uncompression.
func (s *Stream) verifyData(offset int, buf []byte, lfh *localFileHeader, r *bufio.Reader) error {
	var re io.Reader = r
	if lfh.deflated() {
		s.decompressor.(flate.Resetter).Reset(r, nil)
		re = s.decompressor
	}

	crc := crc32.NewIEEE()
	var mw io.Writer = crc

	s.opt.Verifier.Reset()
	mw = io.MultiWriter(crc, s.opt.Verifier)

	n, err := io.CopyN(mw, re, int64(lfh.uncompressedSize))
	if err != nil {
		return err
	}
	if n != int64(lfh.uncompressedSize) {
		if err != nil {
			return s.corruptError(-1, "unable to read out contents: "+err.Error())
		}
		return s.corruptError(-1, "specified contents extends beyond file")
	}

	if crc.Sum32() != lfh.cRC {
		return s.corruptError(offset+14, "crc not matching stored data")
	}
	if s.opt.Verifier != nil && !s.opt.Verifier.Match(lfh.fileName) {
		return s.corruptError(offset+30, "filename not matching the expected: "+lfh.fileName)
	}
	return nil
}

func (s *Stream) writeDirectory(fileLength int) error {
	// ensure that if a directory was previously written we overwrite it all with zeros
	// if the new one is shorted (at a wipe) to ensure we are not leaking data. We do this
	// by reading backwards block wise to wipe the data, then re-writing.
	// expects caller to hold a write lock.

	s.lastAppend = false
	s.lastRead = -2

	if fileLength != 0 {
		// this is the amount of extra data that might need to be overwritten
		// to ensure the file name is removed at a wipe.

		fileLength += 46
	}

	if err := s.sync(); err != nil {
		return nil
	}

	oldLen, newLen := fileLength, 24
	for _, e := range s.entries {
		newLen += 46 + len(e.Name)
	}
	oldLen += newLen

	if _, err := s.file.Seek(int64(s.opt.FileSize)-int64(oldLen), 0); err != nil {
		return err
	}

	if oldLen > newLen {
		for i := range s.buffer {
			s.buffer[i] = 0
			if i > (oldLen - newLen) {
				break
			}
		}
		var b []byte
		for toWipe := oldLen - newLen; toWipe > 0; toWipe -= len(b) {
			if toWipe > len(s.buffer) {
				b = s.buffer
			} else {
				b = s.buffer[:toWipe]
			}
			_, err := s.file.Write(b)
			if err != nil {
				return err
			}
		}
	}

	// so we are now ready to write out the new central directory record
	w := bufio.NewWriter(s.file)
	for _, e := range s.entries {
		w.Write(encodeDirectoryHeader(s.buffer,
			false,
			e.crc,
			e.CompressedSize,
			e.UncompressedSize,
			e.Name, e.modTime, e.modDate, e.Offset-30-int32(len(e.Name))))
	}

	// finally write out the eocd record to end the file
	w.Write(directoryEndStream)
	binary.LittleEndian.PutUint16(s.buffer[4-4:], 0)
	binary.LittleEndian.PutUint16(s.buffer[6-4:], 0)
	binary.LittleEndian.PutUint16(s.buffer[8-4:], uint16(len(s.entries)))
	binary.LittleEndian.PutUint16(s.buffer[10-4:], uint16(len(s.entries)))
	binary.LittleEndian.PutUint32(s.buffer[12-4:], uint32(newLen-24))

	if len(s.entries) > 0 {
		binary.LittleEndian.PutUint32(s.buffer[16-4:], uint32(s.opt.FileSize)-uint32(newLen))

	} else {
		binary.LittleEndian.PutUint32(s.buffer[16-4:], 0)
	}
	binary.LittleEndian.PutUint16(s.buffer[20-4:], 2)
	binary.LittleEndian.PutUint16(s.buffer[22-4:], 0)
	w.Write(s.buffer[:22-4+2])

	return w.Flush()
}
