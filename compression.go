package mimic

import (
	"bufio"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

// decodeResponse only handles responses negotiated by the profile. Validate the
// whole encoding stack before changing metadata; unknown encodings remain raw.
func decodeResponse(resp *http.Response) {
	if resp.Body == nil || resp.Body == http.NoBody || resp.StatusCode == http.StatusPartialContent ||
		resp.StatusCode < 200 || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotModified {
		return
	}
	value := strings.Join(resp.Header.Values("Content-Encoding"), ",")
	if value == "" {
		return
	}
	encodings := strings.Split(strings.ToLower(value), ",")
	for i, encoding := range encodings {
		encodings[i] = strings.TrimSpace(encoding)
		switch encodings[i] {
		case "gzip", "deflate", "br", "zstd":
		default:
			return
		}
	}
	resp.Body = &decodedBody{
		body:      resp.Body,
		encodings: encodings,
	}
	resp.Header.Del("Content-Encoding")
	resp.Header.Del("Content-Length")
	resp.ContentLength = -1
	resp.Uncompressed = true
}

// decodedBody initializes codecs on the first Read, so RoundTrip never waits for
// compressed body bytes. Closing the underlying body first unblocks a concurrent
// Read before we acquire mu and release the codecs.
type decodedBody struct {
	body      io.ReadCloser
	encodings []string
	mu        sync.Mutex
	reader    io.Reader
	closers   []io.Closer
	initErr   error
	closed    bool
	closeOnce sync.Once
	closeErr  error
}

func (b *decodedBody) Read(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return 0, http.ErrBodyReadAfterClose
	}
	if b.reader == nil && b.initErr == nil {
		b.reader = b.body
		for i := len(b.encodings) - 1; i >= 0; i-- {
			reader, err := contentDecoder(b.encodings[i], b.reader)
			if err != nil {
				b.initErr = err
				break
			}
			b.reader = reader
			b.closers = append(b.closers, reader)
		}
	}
	if b.initErr != nil {
		return 0, b.initErr
	}

	return b.reader.Read(p)
}

func (b *decodedBody) Close() error {
	b.closeOnce.Do(func() {
		b.closeErr = b.body.Close()
		b.mu.Lock()
		defer b.mu.Unlock()
		b.closed = true
		for i := len(b.closers) - 1; i >= 0; i-- {
			_ = b.closers[i].Close()
		}
		b.reader = nil
		b.closers = nil
	})

	return b.closeErr
}

// contentReader owns one coding boundary. Codec EOF is only successful after
// its entire input also reaches EOF, recursively validating outer codecs and
// HTTP framing/trailers. Keep errors sticky: an inner codec may consume bytes
// returned alongside an error without propagating that error on the same read.
type contentReader struct {
	input   *bufio.Reader
	decoder io.Reader
	err     error
}

func (r *contentReader) Read(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	n, err := r.decoder.Read(p)
	if err == io.EOF {
		_, err = r.input.ReadByte()
		if err == nil {
			err = errors.New("mimic: trailing data after content encoding")
		}
	}
	r.err = err

	return n, err
}

func (r *contentReader) Close() error {
	// Only release this codec; decodedBody owns the response and other layers.
	if closer, ok := r.decoder.(io.Closer); ok {
		return closer.Close()
	}

	return nil
}

func contentDecoder(encoding string, input io.Reader) (io.ReadCloser, error) {
	// All codecs share an observable input boundary. In particular, deflate
	// must not hide buffered bytes or errors in a private read-ahead buffer.
	buffered := bufio.NewReader(input)
	var reader io.Reader
	var err error
	switch encoding {
	case "gzip":
		reader, err = gzip.NewReader(buffered)
	case "deflate":
		// HTTP deflate is zlib-wrapped, but browsers also accept raw DEFLATE.
		// Peek without consuming bytes so a raw stream can use the same reader.
		header, readErr := buffered.Peek(2)
		if readErr != nil {
			if readErr == io.EOF {
				readErr = io.ErrUnexpectedEOF
			}
			return nil, readErr
		}
		if header[0]&0x0f == 8 && header[0]>>4 <= 7 && (uint16(header[0])<<8|uint16(header[1]))%31 == 0 {
			reader, err = zlib.NewReader(buffered)
		} else {
			reader = flate.NewReader(buffered)
		}
	case "br":
		reader = brotli.NewReader(buffered)
	case "zstd":
		decoder, err := zstd.NewReader(buffered,
			zstd.WithDecoderConcurrency(1),
			// RFC 9659 limits the HTTP content-coding window to 8 MiB.
			zstd.WithDecoderMaxWindow(8<<20),
		)
		if err != nil {
			return nil, err
		}

		reader = decoder.IOReadCloser()
	default:
		return nil, fmt.Errorf("mimic: unsupported content encoding %q", encoding)
	}
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}

	return &contentReader{
		input:   buffered,
		decoder: reader,
	}, nil
}
