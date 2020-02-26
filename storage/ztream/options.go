package ztream

import (
	"compress/flate"
	"errors"
	"io"

	"github.com/imdario/mergo"
)

// A Verifier that is used to check that the data stored in the ztream is stored under
// the same names as the Verifier deterministically would give.
type Verifier interface {
	io.Writer

	Reset()
	// Match should return true if the name provided is a valid name for the data stream written
	// to the Verifier since the last call to Reset.
	Match(name string) bool
}

// Options to configure the ztream.
type Options struct {
	// The size of the zip file that should be allocated when creating a new file. Has no
	// effect when opening an existing ztream.
	FileSize int32
	// If non nill used to verify all exisiting data in a ztream that is opened. Has no
	// effect when creating a new ztream.
	Verifier Verifier
	// If SampleCompressSize > 0 the ztream tries to compress part of any appended data to
	// see if it is worthwile to compress the data to save space. If <= 0 compression is disabled.
	// When compression is enabled the memory usage is increased since a buffer must be maintained
	// for the compressed data to avoid disk seeks.
	SampleCompressSize int
	// Only compress the data if it is shorter than CompressionThreshold*len, else write the
	// uncompressed data.
	CompressionThreshold float32
	// CompressionLevel to use - specified as given by the flate package.
	CompressionLevel int
}

var DefaultOptions = Options{
	FileSize:             1 << 27, // ~250 Mb
	Verifier:             nil,
	SampleCompressSize:   1024 * 4,
	CompressionThreshold: 0.75,
	CompressionLevel:     flate.BestSpeed,
}

func getOptions(opt *Options) error {
	mergo.Merge(opt, DefaultOptions)

	if opt.FileSize < 1<<18 {
		return errors.New("ztream: to small FileSize specified, must be at least 1 << 18")
	}
	if opt.FileSize > 1<<30 {
		return errors.New("zstream: to large FileSize, must be smaller than 1 << 30")
	}

	if opt.SampleCompressSize < 512 {
		return errors.New("ztream: to small SampleCompressSize specified, must be at least 512")
	}
	if opt.SampleCompressSize > 1024*1024 {
		return errors.New("zstream: to large SampleCompressSize, must be smaller than 1Mb")
	}

	if opt.CompressionThreshold < 0.0 {
		return errors.New("ztream: to small CompressionThreshold specified, must be at least 0.0")
	}
	if opt.CompressionThreshold >= 0.999 {
		return errors.New("zstream: to large CompressionThreshold, must be smaller than 0.999")
	}

	if opt.CompressionLevel < -2 {
		return errors.New("ztream: to small CompressionLevel specified, must be at least -2")
	}
	if opt.CompressionLevel >= 9 {
		return errors.New("zstream: to large CompressionLevel, must be smaller than 10")
	}

	return nil
}
