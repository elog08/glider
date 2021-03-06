package vmess

import (
	"bytes"
	"encoding/binary"
	"io"
)

const (
	lenSize          = 2
	maxChunkSize     = 1 << 14 // 16384
	defaultChunkSize = 1 << 13 // 8192
)

type chunkedWriter struct {
	io.Writer
	buf []byte
}

// ChunkedWriter returns a chunked writer
func ChunkedWriter(w io.Writer) io.Writer {
	return &chunkedWriter{
		Writer: w,
		buf:    make([]byte, lenSize+maxChunkSize),
	}
}

func (w *chunkedWriter) Write(b []byte) (int, error) {
	n, err := w.ReadFrom(bytes.NewBuffer(b))
	return int(n), err
}

func (w *chunkedWriter) ReadFrom(r io.Reader) (n int64, err error) {
	for {
		buf := w.buf
		payloadBuf := buf[lenSize : lenSize+defaultChunkSize]

		nr, er := r.Read(payloadBuf)
		if nr > 0 {
			n += int64(nr)
			buf = buf[:lenSize+nr]
			payloadBuf = payloadBuf[:nr]
			binary.BigEndian.PutUint16(buf[:lenSize], uint16(nr))

			_, ew := w.Writer.Write(buf)
			if ew != nil {
				err = ew
				break
			}
		}

		if er != nil {
			if er != io.EOF { // ignore EOF as per io.ReaderFrom contract
				err = er
			}
			break
		}
	}

	return n, err
}

type chunkedReader struct {
	io.Reader
	buf      []byte
	leftover []byte
}

// ChunkedReader returns a chunked reader
func ChunkedReader(r io.Reader) io.Reader {
	return &chunkedReader{
		Reader: r,
		buf:    make([]byte, lenSize+maxChunkSize),
	}
}

func (r *chunkedReader) Read(b []byte) (int, error) {
	if len(r.leftover) > 0 {
		n := copy(b, r.leftover)
		r.leftover = r.leftover[n:]
		return n, nil
	}

	// get length
	_, err := io.ReadFull(r.Reader, r.buf[:lenSize])
	if err != nil {
		return 0, err
	}

	// if length == 0, then this is the end
	len := binary.BigEndian.Uint16(r.buf[:lenSize])
	if len == 0 {
		return 0, nil
	}

	// get payload
	_, err = io.ReadFull(r.Reader, r.buf[:len])
	if err != nil {
		return 0, err
	}

	m := copy(b, r.buf[:len])
	if m < int(len) {
		r.leftover = r.buf[m:len]
	}

	return m, err
}
