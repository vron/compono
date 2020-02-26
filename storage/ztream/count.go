package ztream

import "io"

type countWriter struct {
	count int
	w     io.Writer
}

func (cw *countWriter) Write(p []byte) (n int, err error) {
	if cw.w == nil {
		cw.count += len(p)
		return len(p), nil
	}
	n, err = cw.w.Write(p)
	cw.count += n
	return
}

func (cw *countWriter) Size() int {
	return cw.count
}
