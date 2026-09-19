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
	// 1. Plain
	plainOut := hashAndWrap("", raw)

	// 2. Gzip (BestCompression)
	var gzBuf bytes.Buffer
	gzWriter, err := gzip.NewWriterLevel(&gzBuf, gzip.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("compress: failed to init gzip writer: %w", err)
	}
	if _, err := gzWriter.Write(raw); err != nil {
		return nil, fmt.Errorf("compress: failed writing gzip: %w", err)
	}
	if err := gzWriter.Close(); err != nil {
		return nil, fmt.Errorf("compress: failed closing gzip writer: %w", err)
	}
	gzOut := hashAndWrap(".gz", gzBuf.Bytes())

	// 3. Bzip2 (BestCompression)
	var bz2Buf bytes.Buffer
	bz2Writer, err := bzip2.NewWriter(&bz2Buf, &bzip2.WriterConfig{Level: bzip2.BestCompression})
	if err != nil {
		return nil, fmt.Errorf("compress: failed to init bzip2 writer: %w", err)
	}
	if _, err := bz2Writer.Write(raw); err != nil {
		return nil, fmt.Errorf("compress: failed writing bzip2: %w", err)
	}
	if err := bz2Writer.Close(); err != nil {
		return nil, fmt.Errorf("compress: failed closing bzip2 writer: %w", err)
	}
	bz2Out := hashAndWrap(".bz2", bz2Buf.Bytes())

	// 4. XZ (Level 6 standard)
	var xzBuf bytes.Buffer
	xzWriter, err := xz.NewWriter(&xzBuf)
	if err != nil {
		return nil, fmt.Errorf("compress: failed to init xz writer: %w", err)
	}
	if _, err := xzWriter.Write(raw); err != nil {
		return nil, fmt.Errorf("compress: failed writing xz: %w", err)
	}
	if err := xzWriter.Close(); err != nil {
		return nil, fmt.Errorf("compress: failed closing xz writer: %w", err)
	}
	xzOut := hashAndWrap(".xz", xzBuf.Bytes())

	return &CompressResult{
		Plain: plainOut,
		Gz:    gzOut,
		Bz2:   bz2Out,
		Xz:    xzOut,
	}, nil
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
