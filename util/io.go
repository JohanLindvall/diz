package util

import "io"

func CopyAndClose(w io.Writer, r io.ReadCloser) (written int64, err error) {
	written, err = io.Copy(w, r)
	if er := r.Close(); err == nil {
		err = er
	}
	return
}
