package util

import (
	"io"
	"os"
	"runtime"
	"strings"
)

var (
	lineEnding = "\n"
)

const (
	windowsLineEnding = "\r\n"
	unixLineEnding    = "\n"
)

func init() {
	if runtime.GOOS == "windows" {
		lineEnding = windowsLineEnding
	} else {
		lineEnding = unixLineEnding
	}
}

func CopyAndClose(w io.Writer, r io.ReadCloser) (written int64, err error) {
	written, err = io.Copy(w, r)
	if er := r.Close(); err == nil {
		err = er
	}
	return
}

func ReadLines(fn string) ([]string, error) {
	if bytes, err := os.ReadFile(fn); err == nil {
		return strings.Split(strings.ReplaceAll(string(bytes), windowsLineEnding, unixLineEnding), unixLineEnding), nil
	} else {
		return nil, err
	}
}

func WriteLines(fn string, data []string) error {
	return os.WriteFile(fn, []byte(strings.Join(data, lineEnding)), 0644)
}
