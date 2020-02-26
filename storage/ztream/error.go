package ztream

import "strconv"

// A CorruptError is returned if any unexpected data is found in the file.
type CorruptError struct {
	File   string
	Offset int
	Err    string
}

func (e *CorruptError) Error() string {
	return e.File + ":" + strconv.Itoa(e.Offset) + " " + e.Err
}

func (s *Stream) corruptError(offset int, e string) error {
	return error(&CorruptError{
		File:   s.file.Name(),
		Offset: offset,
		Err:    e,
	})
}

// A VerifyError is returned if the name stored in the ztream does not match
// the name given by the Verifier.
type VerifyError struct {
}
