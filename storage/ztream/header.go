package ztream

import (
	"bytes"
	"encoding/binary"
	"io"
	"strconv"
	"time"
)

var fileHeaderStream = []byte{0x50, 0x4B, 0x03, 0x04}
var directoryHeaderStream = []byte{0x50, 0x4B, 0x01, 0x02}
var directoryEndStream = []byte{0x50, 0x4B, 0x05, 0x06}

type localFileHeader struct {
	versionExtract    int16
	bitFlag           uint16
	compressionMethod int16
	modificationTime  uint16
	modificationDate  uint16
	cRC               uint32
	compressedSize    int32
	uncompressedSize  int32
	fileNameLength    int16
	fileName          string
}

func encodeDirectoryHeader(
	buf []byte,
	wiped bool,
	cRC uint32,
	compressedSize int32,
	uncompressedSize int32,
	fileName string, time, date uint16, offset int32) []byte {

	header := buf[:]
	header[0] = directoryHeaderStream[0]
	header[1] = directoryHeaderStream[1]
	header[2] = directoryHeaderStream[2]
	header[3] = directoryHeaderStream[3]

	binary.LittleEndian.PutUint16(header[4:], 20)
	binary.LittleEndian.PutUint16(header[6:], 20)
	binary.LittleEndian.PutUint16(header[8:], 1<<11)
	if compressedSize == uncompressedSize {
		binary.LittleEndian.PutUint16(header[10:], 0)
	} else {
		binary.LittleEndian.PutUint16(header[10:], 8)
	}
	binary.LittleEndian.PutUint16(header[12:], time)
	binary.LittleEndian.PutUint16(header[14:], date)
	binary.LittleEndian.PutUint32(header[16:], cRC)
	binary.LittleEndian.PutUint32(header[20:], uint32(compressedSize))
	binary.LittleEndian.PutUint32(header[24:], uint32(uncompressedSize))
	binary.LittleEndian.PutUint16(header[28:], uint16(len(fileName)))
	binary.LittleEndian.PutUint16(header[30:], 0)
	binary.LittleEndian.PutUint16(header[32:], 0)
	binary.LittleEndian.PutUint16(header[34:], 0)
	binary.LittleEndian.PutUint16(header[36:], 0)
	binary.LittleEndian.PutUint32(header[38:], 0)
	binary.LittleEndian.PutUint32(header[42:], uint32(offset))
	fn := []byte(fileName)
	for i := range fn {
		header[46+i] = fn[i]
	}

	return header[:46+len(fn)]
}

func encodeFileHeader(
	buf []byte,
	wiped bool,
	cRC uint32,
	compressedSize int32,
	uncompressedSize int32,
	fileName string) ([]byte, uint16, uint16) {

	header := buf[:]
	header[0] = fileHeaderStream[0]
	header[1] = fileHeaderStream[1]
	header[2] = fileHeaderStream[2]
	header[3] = fileHeaderStream[3]

	if wiped {
		binary.LittleEndian.PutUint16(header[4:], 10)
	} else {
		binary.LittleEndian.PutUint16(header[4:], 20)
	}
	binary.LittleEndian.PutUint16(header[6:], 1<<11)
	if compressedSize == uncompressedSize {
		binary.LittleEndian.PutUint16(header[8:], 0)
	} else {
		binary.LittleEndian.PutUint16(header[8:], 8)
	}
	date, time := timeToMsDosTime(time.Now())
	if wiped {
		date, time = 0, 0 // do not encode info about when it was deleted.
	}
	binary.LittleEndian.PutUint16(header[10:], time)
	binary.LittleEndian.PutUint16(header[12:], date)
	binary.LittleEndian.PutUint32(header[14:], cRC)
	binary.LittleEndian.PutUint32(header[18:], uint32(compressedSize))
	binary.LittleEndian.PutUint32(header[22:], uint32(uncompressedSize))
	binary.LittleEndian.PutUint16(header[26:], uint16(len(fileName)))
	binary.LittleEndian.PutUint16(header[28:], 0)
	fn := []byte(fileName)
	for i := range fn {
		header[30+i] = fn[i]
	}

	return header[:30+len(fn)], time, date
}

// copied from go standard library zip package
func timeToMsDosTime(t time.Time) (fDate uint16, fTime uint16) {
	fDate = uint16(t.Day() + int(t.Month())<<5 + (t.Year()-1980)<<9)
	fTime = uint16(t.Second()/2 + t.Minute()<<5 + t.Hour()<<11)
	return
}

// decoding, verifying, ensurting not to read past the header data of the reader.
func (s *Stream) decodeFileHeader(offset int, buf []byte, r io.Reader) (lfh *localFileHeader, err error) {
	n, err := io.ReadFull(r, buf[:30])
	if err == io.EOF {
		return nil, s.corruptError(offset+n, "found EOF when looking for file header")
	}
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(buf[:4], fileHeaderStream) {
		return nil, nil // a local file header was not found here
	}
	lfh = &localFileHeader{}
	lfh.versionExtract = int16(binary.LittleEndian.Uint16(buf[4:]))
	if lfh.versionExtract != 10 && lfh.versionExtract != 20 {
		return nil, s.corruptError(offset+4, "unexpected versionExtract: "+strconv.Itoa(int(lfh.versionExtract)))
	}

	lfh.bitFlag = binary.LittleEndian.Uint16(buf[6:])
	if lfh.bitFlag != 1<<11 {
		return nil, s.corruptError(offset+6, "expected unicode flag only: "+strconv.Itoa(int(lfh.bitFlag)))
	}

	lfh.compressionMethod = int16(binary.LittleEndian.Uint16(buf[8:]))
	if lfh.compressionMethod != 0 && lfh.compressionMethod != 8 {
		return nil, s.corruptError(offset+8, "expected no compression or deflate: "+strconv.Itoa(int(lfh.compressionMethod)))
	}

	lfh.modificationTime = uint16(binary.LittleEndian.Uint16(buf[10:]))
	lfh.modificationDate = uint16(binary.LittleEndian.Uint16(buf[12:]))
	lfh.cRC = binary.LittleEndian.Uint32(buf[14:])

	lfh.compressedSize = int32(binary.LittleEndian.Uint32(buf[18:]))
	if lfh.compressedSize <= 0 {
		return nil, s.corruptError(offset+18, "expected compressed size > 0, got: "+strconv.Itoa(int(lfh.compressedSize)))
	}

	lfh.uncompressedSize = int32(binary.LittleEndian.Uint32(buf[22:]))
	if lfh.uncompressedSize < 0 {
		return nil, s.corruptError(offset+18, "expected uncompressedSize size >= 0, got: "+strconv.Itoa(int(lfh.uncompressedSize)))
	}
	if lfh.deflated() {
		if lfh.compressedSize >= lfh.uncompressedSize {
			return nil, s.corruptError(offset+22, "compressed data >= uncompressed data stored")
		}
	} else {
		if lfh.compressedSize != lfh.uncompressedSize {
			return nil, s.corruptError(offset+22, "uncompressed data must have same sizes")
		}
	}

	lfh.fileNameLength = int16(binary.LittleEndian.Uint16(buf[26:]))
	if lfh.fileNameLength > maxNameLength {
		return nil, s.corruptError(offset+26, "filename length to long: "+strconv.Itoa(int(lfh.fileNameLength)))
	}

	n, err = io.ReadFull(r, buf[:lfh.fileNameLength])
	if err == io.EOF {
		return nil, s.corruptError(offset+n+30, "found EOF when reading filename")
	}
	if err != nil {
		return nil, err
	}

	lfh.fileName = string(buf[:int(lfh.fileNameLength)])

	return lfh, nil
}

func (lfh *localFileHeader) wiped() bool {
	// we (ab-use) the versionExtract field for indicating a wiped range
	return lfh.versionExtract == 10
}

func (lfh *localFileHeader) deflated() bool {
	return lfh.compressionMethod == 8
}
