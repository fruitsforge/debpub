package debian

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/blakesmith/ar"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// DebPackage represents an inspected Debian package binary file.
type DebPackage struct {
	Control PackageControl
	Size    int64
	MD5     string
	SHA1    string
	SHA256  string
	SHA512  string
}

type byteCountingWriter struct {
	total int64
}

func (c *byteCountingWriter) Write(p []byte) (int, error) {
	c.total += int64(len(p))
	return len(p), nil
}

// ParseDebReader extracts control metadata and hashes from an io.Reader of a .deb file without buffering the entire archive into memory.
func ParseDebReader(r io.Reader) (*DebPackage, error) {
	counter := &byteCountingWriter{}
	md5H := md5.New()
	sha1H := sha1.New()
	sha256H := sha256.New()
	sha512H := sha512.New()

	mw := io.MultiWriter(counter, md5H, sha1H, sha256H, sha512H)
	tee := io.TeeReader(r, mw)

	arReader := ar.NewReader(tee)
	var controlData []byte

	for {
		header, err := arReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("corrupt ar archive in .deb: %w", err)
		}

		name := strings.TrimRight(header.Name, "/")
		if strings.HasPrefix(name, "control.tar") {
			var errExtract error
			controlData, errExtract = extractControlTar(arReader, name)
			if errExtract != nil {
				return nil, errExtract
			}
			break
		}
	}

	if len(controlData) == 0 {
		return nil, errors.New("control file not found inside .deb archive")
	}

	// Drain remainder of the archive to ensure hashes and byte count cover the entire .deb
	if _, err := io.Copy(io.Discard, tee); err != nil {
		return nil, fmt.Errorf("failed reading remaining .deb data: %w", err)
	}

	paras, err := ParseParagraphs(bytes.NewReader(controlData))
	if err != nil || len(paras) == 0 {
		return nil, fmt.Errorf("failed to parse control paragraph: %w", err)
	}

	stanza := ParagraphToStanza(paras[0])

	return &DebPackage{
		Control: stanza.PackageControl,
		Size:    counter.total,
		MD5:     hex.EncodeToString(md5H.Sum(nil)),
		SHA1:    hex.EncodeToString(sha1H.Sum(nil)),
		SHA256:  hex.EncodeToString(sha256H.Sum(nil)),
		SHA512:  hex.EncodeToString(sha512H.Sum(nil)),
	}, nil
}

func extractControlTar(r io.Reader, name string) ([]byte, error) {
	var tarReader *tar.Reader
	var closer io.Closer

	switch {
	case strings.HasSuffix(name, ".gz"):
		gz, err := gzip.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress %s: %w", name, err)
		}
		closer = gz
		tarReader = tar.NewReader(gz)
	case strings.HasSuffix(name, ".xz"):
		xzR, err := xz.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress %s: %w", name, err)
		}
		tarReader = tar.NewReader(xzR)
	case strings.HasSuffix(name, ".zst"):
		zstdR, err := zstd.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress %s: %w", name, err)
		}
		closer = zstdR.IOReadCloser()
		tarReader = tar.NewReader(zstdR)
	default:
		tarReader = tar.NewReader(r)
	}

	if closer != nil {
		defer func() { _ = closer.Close() }()
	}

	for {
		tarHeader, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading %s: %w", name, err)
		}
		if strings.TrimPrefix(tarHeader.Name, "./") == "control" {
			data, errRead := io.ReadAll(tarReader)
			if errRead != nil {
				return nil, fmt.Errorf("failed reading control file from tar: %w", errRead)
			}
			return data, nil
		}
	}
	return nil, nil
}
