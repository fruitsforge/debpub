// Package compress generates multi-format Debian index compressions (gzip, bzip2, xz) and computes hashes on the fly.
package compress

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"

	"github.com/dsnet/compress/bzip2"
	"github.com/ulikunitz/xz"
)

// Output represents a generated compression variant with its content and checksums.
type Output struct {
	Ext    string // "", ".gz", ".bz2", ".xz"
	Data   []byte
	Size   int64
	MD5    string
	SHA1   string
	SHA256 string
	SHA512 string
}

// CompressResult contains all compressed streams generated from uncompressed input.
type CompressResult struct {
	Plain Output
	Gz    Output
	Bz2   Output
	Xz    Output
}

// AllOutputs returns all compression outputs as a slice for iteration.
func (r *CompressResult) AllOutputs() []Output {
	return []Output{r.Plain, r.Gz, r.Bz2, r.Xz}
}

// CompressAll streams raw bytes into plain, gzip, bzip2, and xz variants,
// computing all hashes (MD5, SHA1, SHA256, SHA512) and sizes on the fly.
func CompressAll(raw []byte) (*CompressResult, error) {
	gzOut, err := compressGzip(raw)
	if err != nil {
		return nil, err
	}

	bz2Out, err := compressBzip2(raw)
	if err != nil {
		return nil, err
	}

	xzOut, err := compressXZ(raw)
	if err != nil {
		return nil, err
	}

	return &CompressResult{
		Plain: hashAndWrap("", raw),
		Gz:    gzOut,
		Bz2:   bz2Out,
		Xz:    xzOut,
	}, nil
}

func compressGzip(raw []byte) (out Output, err error) {
	var buf bytes.Buffer
	w, initErr := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if initErr != nil {
		return Output{}, fmt.Errorf("compress: failed to init gzip writer: %w", initErr)
	}
	defer func() {
		if cerr := w.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("compress: failed closing gzip writer: %w", cerr)
		}
		if err == nil {
			out = hashAndWrap(".gz", buf.Bytes())
		}
	}()

	if _, err = w.Write(raw); err != nil {
		return Output{}, fmt.Errorf("compress: failed writing gzip: %w", err)
	}
	return out, nil
}

func compressBzip2(raw []byte) (out Output, err error) {
	var buf bytes.Buffer
	w, initErr := bzip2.NewWriter(&buf, &bzip2.WriterConfig{Level: bzip2.BestCompression})
	if initErr != nil {
		return Output{}, fmt.Errorf("compress: failed to init bzip2 writer: %w", initErr)
	}
	defer func() {
		if cerr := w.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("compress: failed closing bzip2 writer: %w", cerr)
		}
		if err == nil {
			out = hashAndWrap(".bz2", buf.Bytes())
		}
	}()

	if _, err = w.Write(raw); err != nil {
		return Output{}, fmt.Errorf("compress: failed writing bzip2: %w", err)
	}
	return out, nil
}

func compressXZ(raw []byte) (out Output, err error) {
	var buf bytes.Buffer
	w, initErr := xz.NewWriter(&buf)
	if initErr != nil {
		return Output{}, fmt.Errorf("compress: failed to init xz writer: %w", initErr)
	}
	defer func() {
		if cerr := w.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("compress: failed closing xz writer: %w", cerr)
		}
		if err == nil {
			out = hashAndWrap(".xz", buf.Bytes())
		}
	}()

	if _, err = w.Write(raw); err != nil {
		return Output{}, fmt.Errorf("compress: failed writing xz: %w", err)
	}
	return out, nil
}

func hashAndWrap(ext string, data []byte) Output {
	return Output{
		Ext:    ext,
		Data:   data,
		Size:   int64(len(data)),
		MD5:    sum(md5.New(), data),
		SHA1:   sum(sha1.New(), data),
		SHA256: sum(sha256.New(), data),
		SHA512: sum(sha512.New(), data),
	}
}

func sum(h hash.Hash, data []byte) string {
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
