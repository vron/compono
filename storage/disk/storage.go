package diskstorage

import (
	"context"
	"io"
	"time"

	"github.com/vron/compono/blob"
)

type Storage struct {
}

func (s *Storage) FetchBlob(context.Context, blob.Ref) (blob io.ReadCloser, size uint32, err error) {
	panic("NIY")
}

func (s *Storage) ReceiveBlob(ctx context.Context, br, fl blob.Ref, source io.Reader) (blob.SizedRef, error) {
	panic("NIY")

}

func (s *Storage) StatBlobs(ctx context.Context, blobs []blob.Ref, fn func(blob.SizedRef) error) error {
	panic("NIY")

}

func (s *Storage) EnumerateBlobs(ctx context.Context,
	dest chan<- blob.SizedRef,
	after string) error {
	panic("NIY")

}

func (s *Storage) RemoveBlobs(ctx context.Context, blobs []blob.Ref) error {
	panic("NIY")

}

func (s *Storage) StorageGeneration() (initTime time.Time, random string, err error) {
	panic("NIY")

}

func (s *Storage) ResetStorageGeneration() error {
	panic("NIY")

}

func (s *Storage) Close() error {
	panic("NIY")

}
