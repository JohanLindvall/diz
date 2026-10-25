// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Adapted from klauspost/pgzip's gzip.go:
//   https://github.com/klauspost/pgzip/blob/master/gzip.go
// (re-synced against upstream master on 2026-07-10).
//
// pgzip is a parallel GZIP writer. This file keeps pgzip's block-parallel
// compression engine intact but strips the GZIP framing, so the output is a raw
// DEFLATE stream — which is what klauspost/compress/zip's RegisterCompressor
// expects. Every place this fork diverges from upstream is tagged with a
// "diz:" comment so the file can be re-synced against a newer pgzip cheaply.
//
// Summary of divergences from upstream gzip.go:
//   - Renamed Writer -> DeflateWriter and NewWriter[Level] ->
//     NewDeflateWriter[Level], to avoid colliding with hashzip.Writer and to
//     make clear that no GZIP framing is emitted.
//   - Dropped the GZIP header: no embedded Header, no 10-byte header written in
//     Write, and none of put2/put4/writeBytes/writeString or the gzip* id
//     constants (those live in pgzip's gunzip.go).
//   - Dropped the GZIP trailer (CRC32 + ISIZE) written in Close, and with it the
//     hash.Hash32/crc32 digest field and the [10]byte scratch buffer that only
//     existed to build the header and trailer.
//
// The parallel engine itself — block splitting, one goroutine per block, the
// ordered fan-in writer, the sync.Pool reuse, and prevTail dictionary priming —
// is copied verbatim.

package hashzip

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"

	"github.com/klauspost/compress/flate"
)

const (
	defaultBlockSize = 1 << 20
	tailSize         = 16384
	defaultBlocks    = 4
)

// These constants are copied from the flate package, so that code that imports
// "compress/gzip" does not also have to import "compress/flate".
const (
	NoCompression       = flate.NoCompression
	BestSpeed           = flate.BestSpeed
	BestCompression     = flate.BestCompression
	DefaultCompression  = flate.DefaultCompression
	ConstantCompression = flate.ConstantCompression
	HuffmanOnly         = flate.HuffmanOnly
)

// A DeflateWriter is an io.WriteCloser. Writes to it are compressed with
// pgzip's parallel engine and written to w as a raw DEFLATE stream.
//
// diz: upstream's type is Writer and embeds a gzip Header; this fork drops the
// Header, the crc32 digest, and the [10]byte scratch buffer that fed the GZIP
// header and trailer.
type DeflateWriter struct {
	w             io.Writer
	level         int
	wroteHeader   bool // diz: retained only to gate one-time collector startup
	blockSize     int
	blocks        int
	currentBuffer []byte
	prevTail      []byte
	size          int
	closed        bool
	errMu         sync.RWMutex
	err           error
	pushedErr     chan struct{}
	results       chan result
	dictFlatePool sync.Pool
	dstPool       sync.Pool
	wg            sync.WaitGroup
}

type result struct {
	result        chan []byte
	notifyWritten chan struct{}
}

// SetConcurrency finetunes the concurrency level if needed.
//
// With this you can control the approximate size of your blocks,
// as well as how many you want to be processing in parallel.
//
// Default values for this is SetConcurrency(defaultBlockSize, runtime.GOMAXPROCS(0)),
// meaning blocks are split at 1 MB and up to the number of CPU threads
// can be processing at once before the writer blocks.
func (z *DeflateWriter) SetConcurrency(blockSize, blocks int) error {
	if blockSize <= tailSize {
		return fmt.Errorf("gzip: block size cannot be less than or equal to %d", tailSize)
	}
	if blocks <= 0 {
		return errors.New("gzip: blocks cannot be zero or less")
	}
	if blockSize == z.blockSize && blocks == z.blocks {
		return nil
	}
	z.blockSize = blockSize
	z.results = make(chan result, blocks)
	z.blocks = blocks
	z.dstPool.New = func() any { return make([]byte, 0, blockSize+(blockSize)>>4) }
	return nil
}

// NewDeflateWriter returns a new DeflateWriter.
// Writes to the returned writer are compressed and written to w.
//
// It is the caller's responsibility to call Close on the WriteCloser when done.
// Writes may be buffered and not flushed until Close.
func NewDeflateWriter(w io.Writer) *DeflateWriter {
	z, _ := NewDeflateWriterLevel(w, DefaultCompression)
	return z
}

// NewDeflateWriterLevel is like NewDeflateWriter but specifies the compression
// level instead of assuming DefaultCompression.
//
// The compression level can be DefaultCompression, NoCompression, or any
// integer value between BestSpeed and BestCompression inclusive. The error
// returned will be nil if the level is valid.
func NewDeflateWriterLevel(w io.Writer, level int) (*DeflateWriter, error) {
	if level < ConstantCompression || level > BestCompression {
		return nil, fmt.Errorf("gzip: invalid compression level: %d", level)
	}
	z := new(DeflateWriter)
	z.SetConcurrency(defaultBlockSize, runtime.GOMAXPROCS(0))
	z.init(w, level)
	return z, nil
}

// This function must be used by goroutines to set an
// error condition, since z.err access is restricted
// to the callers goruotine.
func (z *DeflateWriter) pushError(err error) {
	z.errMu.Lock()
	if z.err != nil {
		z.errMu.Unlock()
		return
	}
	z.err = err
	close(z.pushedErr)
	z.errMu.Unlock()
}

func (z *DeflateWriter) init(w io.Writer, level int) {
	z.wg.Wait()
	// diz: upstream also (re)initializes the crc32 digest and the gzip Header
	// fields (OS/Comment/Extra/ModTime) and buf here; none exist in this fork.
	z.w = w
	z.level = level
	z.pushedErr = make(chan struct{}, 0)
	z.results = make(chan result, z.blocks)
	z.err = nil
	z.closed = false
	z.wroteHeader = false
	if z.currentBuffer != nil {
		z.dstPool.Put(z.currentBuffer)
		z.currentBuffer = nil
	}
	if z.prevTail != nil {
		z.dstPool.Put(z.prevTail)
		z.prevTail = nil
	}
	z.size = 0
	if z.dictFlatePool.New == nil {
		z.dictFlatePool.New = func() any {
			f, _ := flate.NewWriterDict(w, level, nil)
			return f
		}
	}
}

// Reset discards the DeflateWriter z's state and makes it equivalent to the
// result of its original state from NewDeflateWriter or NewDeflateWriterLevel,
// but writing to w instead. This permits reusing a writer rather than
// allocating a new one.
func (z *DeflateWriter) Reset(w io.Writer) {
	if z.results != nil && !z.closed {
		close(z.results)
	}
	z.SetConcurrency(defaultBlockSize, runtime.GOMAXPROCS(0))
	z.init(w, z.level)
}

// diz: upstream defines put2/put4/writeBytes/writeString here to build the GZIP
// header and trailer. A raw DEFLATE stream has neither, so they are omitted.

// compressCurrent will compress the data currently buffered
// This should only be called from the main writer/flush/closer
func (z *DeflateWriter) compressCurrent(flush bool) {
	c := z.currentBuffer
	if len(c) > z.blockSize {
		// This can never happen through the public interface.
		panic("len(z.currentBuffer) > z.blockSize (most likely due to concurrent Write race)")
	}

	r := result{}
	r.result = make(chan []byte, 1)
	r.notifyWritten = make(chan struct{}, 0)
	// Reserve a result slot
	select {
	case z.results <- r:
	case <-z.pushedErr:
		return
	}

	z.wg.Add(1)
	tail := z.prevTail
	if len(c) > tailSize {
		buf := z.dstPool.Get().([]byte) // Put in .compressBlock
		// Copy tail from current buffer before handing the buffer over to the
		// compressBlock goroutine.
		buf = append(buf[:0], c[len(c)-tailSize:]...)
		z.prevTail = buf
	} else {
		z.prevTail = nil
	}
	go z.compressBlock(c, tail, r, z.closed)

	z.currentBuffer = z.dstPool.Get().([]byte) // Put in .compressBlock
	z.currentBuffer = z.currentBuffer[:0]

	// Wait if flushing
	if flush {
		<-r.notifyWritten
	}
}

// Returns an error if it has been set.
// Cannot be used by functions that are from internal goroutines.
func (z *DeflateWriter) checkError() error {
	z.errMu.RLock()
	err := z.err
	z.errMu.RUnlock()
	return err
}

// Write writes a compressed form of p to the underlying io.Writer. The
// compressed bytes are not necessarily flushed to output until
// the Writer is closed or Flush() is called.
//
// The function will return quickly, if there are unused buffers.
// The sent slice (p) is copied, and the caller is free to re-use the buffer
// when the function returns.
//
// Errors that occur during compression will be reported later, and a nil error
// does not signify that the compression succeeded (since it is most likely still running)
// That means that the call that returns an error may not be the call that caused it.
// Only Flush and Close functions are guaranteed to return any errors up to that point.
func (z *DeflateWriter) Write(p []byte) (int, error) {
	if err := z.checkError(); err != nil {
		return 0, err
	}
	// diz: upstream writes the 10-byte GZIP header (plus Extra/Name/Comment)
	// here on the first Write. This fork emits raw DEFLATE, so the first Write
	// only starts the goroutine that drains compressed blocks in order.
	if !z.wroteHeader {
		z.wroteHeader = true
		// Start receiving data from compressors
		go func() {
			listen := z.results
			var failed bool
			for {
				r, ok := <-listen
				// If closed, we are finished.
				if !ok {
					return
				}
				if failed {
					close(r.notifyWritten)
					continue
				}
				buf := <-r.result
				n, err := z.w.Write(buf)
				if err != nil {
					z.pushError(err)
					close(r.notifyWritten)
					failed = true
					continue
				}
				if n != len(buf) {
					z.pushError(fmt.Errorf("gzip: short write %d should be %d", n, len(buf)))
					failed = true
					close(r.notifyWritten)
					continue
				}
				z.dstPool.Put(buf)
				close(r.notifyWritten)
			}
		}()
		z.currentBuffer = z.dstPool.Get().([]byte)
		z.currentBuffer = z.currentBuffer[:0]
	}
	q := p
	for len(q) > 0 {
		length := len(q)
		if length+len(z.currentBuffer) > z.blockSize {
			length = z.blockSize - len(z.currentBuffer)
		}
		// diz: upstream updates the crc32 digest here; this fork emits no
		// trailer, so no running checksum is kept.
		z.currentBuffer = append(z.currentBuffer, q[:length]...)
		if len(z.currentBuffer) > z.blockSize {
			panic("z.currentBuffer too large (most likely due to concurrent Write race)")
		}
		if len(z.currentBuffer) == z.blockSize {
			z.compressCurrent(false)
			if err := z.checkError(); err != nil {
				return len(p) - len(q), err
			}
		}
		z.size += length
		q = q[length:]
	}
	return len(p), z.checkError()
}

// Step 1: compresses buffer to buffer
// Step 2: send writer to channel
// Step 3: Close result channel to indicate we are done
func (z *DeflateWriter) compressBlock(p, prevTail []byte, r result, closed bool) {
	defer func() {
		close(r.result)
		z.wg.Done()
	}()
	buf := z.dstPool.Get().([]byte) // Corresponding Put in .Write's result writer
	dest := bytes.NewBuffer(buf[:0])

	compressor := z.dictFlatePool.Get().(*flate.Writer) // Put below
	compressor.ResetDict(dest, prevTail)
	compressor.Write(p)
	z.dstPool.Put(p) // Corresponding Get in .Write and .compressCurrent

	err := compressor.Flush()
	if err != nil {
		z.pushError(err)
		return
	}
	if closed {
		err = compressor.Close()
		if err != nil {
			z.pushError(err)
			return
		}
	}
	z.dictFlatePool.Put(compressor) // Get above

	if prevTail != nil {
		z.dstPool.Put(prevTail) // Get in .compressCurrent
	}

	// Read back buffer
	buf = dest.Bytes()
	r.result <- buf
}

// Flush flushes any pending compressed data to the underlying writer.
//
// It is useful mainly in compressed network protocols, to ensure that
// a remote reader has enough data to reconstruct a packet. Flush does
// not return until the data has been written. If the underlying
// writer returns an error, Flush returns that error.
//
// In the terminology of the zlib library, Flush is equivalent to Z_SYNC_FLUSH.
func (z *DeflateWriter) Flush() error {
	if err := z.checkError(); err != nil {
		return err
	}
	if z.closed {
		return nil
	}
	if !z.wroteHeader {
		_, err := z.Write(nil)
		if err != nil {
			return err
		}
	}
	// We send current block to compression
	z.compressCurrent(true)

	return z.checkError()
}

// UncompressedSize will return the number of bytes written.
// pgzip only, not a function in the official gzip package.
func (z *DeflateWriter) UncompressedSize() int {
	return z.size
}

// Close closes the Writer, flushing any unwritten data to the underlying
// io.Writer, but does not close the underlying io.Writer.
func (z *DeflateWriter) Close() error {
	if err := z.checkError(); err != nil {
		return err
	}
	if z.closed {
		return nil
	}

	z.closed = true
	if !z.wroteHeader {
		z.Write(nil)
		if err := z.checkError(); err != nil {
			return err
		}
	}
	z.compressCurrent(true)
	if err := z.checkError(); err != nil {
		return err
	}
	close(z.results)
	// diz: upstream writes the GZIP trailer (CRC32 + ISIZE) here; a raw DEFLATE
	// stream has no trailer, so Close ends once the last block is drained.
	return nil
}
