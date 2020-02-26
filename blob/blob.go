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
)

// TOOD: Wrong place for these? Or should there be a hard limit for max size?
const (
	MaxSize          = 1 << 20  // ~1Mb
	MinSizeThreshold = 16 << 10 // ~16kB
)

type Blob struct {
	Ref    Ref
	Data   []byte
	Size   uint32
	Schema bool
}

// Valid reports whether the hash of blob's content matches
// its reference.
func (b *Blob) Valid() bool {
	h := b.Ref.Hash()
	h.Write(b.Data)
	return bytes.Equal(h.Sum(nil), b.Ref.ref.bytes())
}
