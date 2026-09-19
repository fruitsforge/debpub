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

// ParseDebReader extracts control metadata and hashes from an io.ReaderAt / io.Reader of a .deb file.
func ParseDebReader(r io.Reader) (*DebPackage, error) {
	// Read entire .deb into memory or buffer for hashing and ar parsing
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read .deb data: %w", err)
	}

	size := int64(len(raw))
	md5Sum := fmt.Sprintf("%x", md5.Sum(raw))
	sha1Sum := fmt.Sprintf("%x", sha1.Sum(raw))
	sha256Sum := fmt.Sprintf("%x", sha256.Sum256(raw))
	sha512SumBytes := sha512.Sum512(raw)
	sha512Sum := hex.EncodeToString(sha512SumBytes[:])

	arReader := ar.NewReader(bytes.NewReader(raw))
	var controlData []byte

	for {
		header, err := arReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("corrupt ar archive in .deb: %w", err)
		}

		name := strings.TrimRight(header.Name, "/")
		if strings.HasPrefix(name, "control.tar") {
			var tarReader *tar.Reader

			if strings.HasSuffix(name, ".gz") {
				gz, err := gzip.NewReader(arReader)
				if err != nil {
					return nil, fmt.Errorf("failed to decompress %s: %w", name, err)
				}
				tarReader = tar.NewReader(gz)
			} else if strings.HasSuffix(name, ".xz") {
				xzR, err := xz.NewReader(arReader)
				if err != nil {
					return nil, fmt.Errorf("failed to decompress %s: %w", name, err)
				}
				tarReader = tar.NewReader(xzR)
			} else if strings.HasSuffix(name, ".zst") {
				zstdR, err := zstd.NewReader(arReader)
				if err != nil {
					return nil, fmt.Errorf("failed to decompress %s: %w", name, err)
				}
				tarReader = tar.NewReader(zstdR)
			} else {
				tarReader = tar.NewReader(arReader)
			}

			// Scan tar entries for "./control" or "control"
			for {
				tarHeader, err := tarReader.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					return nil, fmt.Errorf("error reading %s: %w", name, err)
				}
				tarName := strings.TrimPrefix(tarHeader.Name, "./")
				if tarName == "control" {
					controlData, err = io.ReadAll(tarReader)
					if err != nil {
						return nil, fmt.Errorf("failed reading control file from tar: %w", err)
					}
					break
				}
			}
			break
		}
	}

	if len(controlData) == 0 {
		return nil, fmt.Errorf("control file not found inside .deb archive")
	}

	paras, err := ParseParagraphs(bytes.NewReader(controlData))
	if err != nil || len(paras) == 0 {
		return nil, fmt.Errorf("failed to parse control paragraph: %w", err)
	}

	stanza := ParagraphToStanza(paras[0])

	return &DebPackage{
		Control: stanza.PackageControl,
		Size:    size,
		MD5:     md5Sum,
		SHA1:    sha1Sum,
		SHA256:  sha256Sum,
		SHA512:  sha512Sum,
	}, nil
}
