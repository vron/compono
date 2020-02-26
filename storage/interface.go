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

package interface

// Fetcher is the interface for fetching blobs.
type Fetcher interface {
	// Fetch returns a blob. If the blob is not found then
	// os.ErrNotExist should be returned.
	//
	// The contents are not guaranteed to match the digest of the
	// provided Ref (e.g. when streamed over HTTP). Paranoid
	// callers should verify them.
	//
	// The provided context is used until blob is closed and its
	// cancelation should but may not necessarily cause reads from
	// blob to fail with an error.
}

// Receiver is the interface for receiving blobs.
type Receiver interface {
	// ReceiveBlob accepts a newly uploaded blob and writes it to
	// permanent storage.
	//
	// Implementations of BlobReceiver downstream of the HTTP
	// server can trust that the source isn't larger than
	// MaxBlobSize and that its digest matches the provided blob
	// ref. (If not, the read of the source will fail before EOF)
	//
	// To ensure those guarantees, callers of ReceiveBlob should
	// not call ReceiveBlob directly but instead use either
	// blobserver.Receive or blobserver.ReceiveString, which also
	// take care of notifying the BlobReceiver's "BlobHub"
	// notification bus for observers.
}

// Statter is the interface for checking the size and existence of blobs.
type Statter interface {
	// Stat checks for the existence of blobs, calling fn in
	// serial for each found blob, in any order, but with no
	// duplicates. The blobs slice should not have duplicates.
	//
	// If fn returns an error, StatBlobs returns with that value
	// and makes no further calls to fn.
	//
	// StatBlobs does not return an error on missing blobs, only
	// on failure to stat blobs.
}

type Enumerator interface {
	// EnumerateBobs sends at most limit SizedBlobRef into dest,
	// sorted, as long as they are lexigraphically greater than
	// after (if provided).
	// limit will be supplied and sanity checked by caller.
	// EnumerateBlobs must close the channel.  (even if limit
	// was hit and more blobs remain, or an error is returned, or
	// the ctx is canceled)
}

type BlobRemover interface {
	// RemoveBlobs removes 0 or more blobs. Removal of
	// non-existent items isn't an error. Returns failure if any
	// items existed but failed to be deleted.
	// If RemoveBlobs returns an error, it's possible that either
	// none or only some of the blobs were deleted.
}

/*
Generationer is an optional interface and an optimization and paranoia
facility for clients which can be implemented by Storage
implementations.
If the client sees the same random string in multiple upload sessions,
it assumes that the blobserver still has all the same blobs, and also
it's the same server.  This mechanism is not fundamental to
Perkeep's operation: the client could also check each blob before
uploading, or enumerate all blobs from the server too.  This is purely
an optimization so clients can mix this value into their "is this file
uploaded?" local cache keys.
*/
type Generationer interface {
	// Generation returns a Storage's initialization time and
	// and unique random string (or UUID).  Implementations
	// should call ResetStorageGeneration on demand if no
	// information is known.
	// The error will be of type GenerationNotSupportedError if an underlying
	// storage target doesn't support the Generationer interface.
	StorageGeneration() (initTime time.Time, random string, err error)

	// ResetGeneration deletes the information returned by Generation
	// and re-generates it.
	ResetStorageGeneration() error
}


// stretch cases to think about
/*

 - long series of very small (e.g. 200 byte schema blobs) - this is important since affects re-indexing etc. etc
 - streaming a very large file - e.g. a movie


*/

// search is very important


// can we do compression client side? of schema? - e.g possibility to seperate the storages?


// Storage is the interface that must be implemented by a blobserver
// storage type.
type Storage interface {
	Close() error

	GetBlobs(context.Context, []blob.Ref) (blob io.ReadCloser, size uint32, err error)

	PutBlob(ctx context.Context, br, after, blob.Ref, source io.Reader) (blob.SizedRef, error)

	StatBlobs(ctx context.Context, blobs []blob.Ref, fn func(blob.SizedRef) error) error

	EnumerateBlobs(ctx context.Context,
		dest chan<- blob.SizedRef,
		filter Filter) error


	RemoveBlobs(ctx context.Context, blobs []blob.Ref) error
}

type BlobGetter interface {
	Next() (blob blob.SizedRef, data io.ReadCloser)
}

type BlobReceiver interface {
	PutBlob()
}

var ErrPending = errors.New("the blob is pending sync to durable storage")

type Filter struct {
	// This one is tricky since we sort schema first
	After string

	ExcludeSchemaBlobs bool
	ExcludeDataBlobs bool
}