/*
Copyright 2020 The Perkeep Authors and The compono authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package blob

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"fmt"
	"hash"
	"strings"
)

// A Ref is reference to a Blob. It must support equality.
type Ref struct {
	hash   [28]byte
	schema bool
}

// SizedRef is a ref with size info in additon.
type SizedRef struct {
	size uint32
	Ref
}

// String formats the Ref as a string.
func (r Ref) String() string {
	buf := getBuf(4 + 1 + 28*2)[:0]
	ref := string(r.appendString("sha2", r.hash[:], buf))
	putBuf(buf)
	return ref
}

// HashName returns the name of the hash function used.
func (r Ref) HashName() string {
	return "sha2"
}

// Hash returns a hash.Hash that can be used to verify this Ref's hash.
func (r Ref) Hash() hash.Hash {
	return sha512.New512_224()
}

// Less reports whether r sorts before o. Schema blobs sort first.
func (r Ref) Less(o Ref) bool {
	// must sort in same order as if sorted on string representation
	if r.schema != o.schema {
		return r.schema
	}
	return bytes.Compare(r.hash[:], o.hash[:]) < 0
}

// Parse a ref from a string.
func Parse(s []byte) (ref Ref, ok bool) {
	if len(s) != 1+1+4+1+28*2 {
		return
	}
	if s[0] == 'S' {
		ref.schema = true
	} else if s[0] == 'd' {
	} else {
		return
	}
	if string(s[1:7]) != ":sha2-" {
		return
	}
	hex := s[7:]
	ok := hexBytes(ref.hash[:], hex)
	if !ok {
		return
	}
	ref.size
	return ref, true
}

func hexVal(b byte, bad *bool) byte {
	if '0' <= b && b <= '9' {
		return b - '0'
	}
	if 'a' <= b && b <= 'f' {
		return b - 'a' + 10
	}
	*bad = true
	return 0
}

func hexBytes(d []byte, hex []byte) bool {
	var bad bool
	for i := 0; i < len(hex); i += 2 {
		d[i/2] = hexVal(hex[i], &bad)<<4 | hexVal(hex[i+1], &bad)
	}
	return !bad
}

var null = []byte(`null`)

// UnmarshalJSON implements encoding/json
func (r *Ref) UnmarshalJSON(d []byte) error {
	if r.ref != nil {
		return errors.New("Can't UnmarshalJSON into a non-zero Ref")
	}
	if len(d) == 0 || bytes.Equal(d, null) {
		return nil
	}
	if len(d) < 2 || d[0] != '"' || d[len(d)-1] != '"' {
		return fmt.Errorf("blob: expecting a JSON string to unmarshal, got %q", d)
	}
	d = d[1 : len(d)-1]
	p, ok := ParseBytes(d)
	if !ok {
		return fmt.Errorf("blobref: invalid blobref %q (%d)", d, len(d))
	}
	*r = p
	return nil
}

// MarshalJSON implements encoding/json
func (r Ref) MarshalJSON() ([]byte, error) {
	if !r.Valid() {
		return null, nil
	}
	dname := r.ref.hash()
	bs := r.ref.bytes()
	buf := make([]byte, 0, 3+len(dname)+len(bs)*2)
	buf = append(buf, '"')
	buf = r.appendString(dname, bs, buf)
	buf = append(buf, '"')
	return buf, nil
}

func (r Ref) appendString(dname string, bs, buf []byte) []byte {
	buf = append(buf, dname...)
	buf = append(buf, '-')
	for _, b := range bs {
		buf = append(buf, hexDigit[b>>4], hexDigit[b&0xf])
	}
	return buf
}

type refType interface {
	bytes() []byte
	hash() string
	newHash() hash.Hash
	equalString(s string) bool
	hasPrefix(s string) bool
}

var bufPool = make(chan []byte, 80)

func getBuf(size int) []byte {
	for {
		select {
		case b := <-bufPool:
			if cap(b) >= size {
				return b[:size]
			}
		default:
			return make([]byte, size)
		}
	}
}

func putBuf(b []byte) {
	select {
	case bufPool <- b:
	default:
	}
}

const hexDigit = "0123456789abcdef"

type sha224Digest [28]byte

const sha224StrLen = 63

func (d sha224Digest) hash() string       { return "sha224" }
func (d sha224Digest) bytes() []byte      { return d[:] }
func (d sha224Digest) newHash() hash.Hash { return sha256.New224() }
func (d sha224Digest) equalString(s string) bool {
	if len(s) != sha224StrLen {
		return false
	}
	if !strings.HasPrefix(s, "sha224-") {
		return false
	}
	s = s[len("sha224-"):]
	for i, b := range d[:] {
		if s[i*2] != hexDigit[b>>4] || s[i*2+1] != hexDigit[b&0xf] {
			return false
		}
	}
	return true
}

func (d sha224Digest) hasPrefix(s string) bool {
	if len(s) > sha224StrLen {
		return false
	}
	if len(s) == sha224StrLen {
		return d.equalString(s)
	}
	if !strings.HasPrefix(s, "sha224-") {
		return false
	}
	s = s[len("sha224-"):]
	if len(s) == 0 {
		// we want at least one digest char to match on
		return false
	}
	for i, b := range d[:] {
		even := i * 2
		if even == len(s) {
			break
		}
		if s[even] != hexDigit[b>>4] {
			return false
		}
		odd := i*2 + 1
		if odd == len(s) {
			break
		}
		if s[odd] != hexDigit[b&0xf] {
			return false
		}
	}
	return true
}
