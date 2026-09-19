package compress

import (
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"io"
	"testing"

	"github.com/ulikunitz/xz"
)

func TestCompressAll(t *testing.T) {
	testData := []byte("Package: test-package\nVersion: 1.0.0\nArchitecture: amd64\nDescription: A sample package index entry.\n\n")

	res, err := CompressAll(testData)
	if err != nil {
		t.Fatalf("CompressAll failed: %v", err)
	}

	// 1. Check Plain
	if !bytes.Equal(res.Plain.Data, testData) {
		t.Errorf("Plain data mismatch")
	}
	if res.Plain.Ext != "" {
		t.Errorf("expected empty ext for plain, got %q", res.Plain.Ext)
	}
	if res.Plain.Size != int64(len(testData)) {
		t.Errorf("expected size %d, got %d", len(testData), res.Plain.Size)
	}

	// 2. Decompress & Check Gzip
	gzReader, err := gzip.NewReader(bytes.NewReader(res.Gz.Data))
	if err != nil {
		t.Fatalf("failed to read gzip: %v", err)
	}
	gzDecompressed, err := io.ReadAll(gzReader)
	if err != nil {
		t.Fatalf("failed to decompress gzip: %v", err)
	}
	if !bytes.Equal(gzDecompressed, testData) {
		t.Errorf("gzip decompressed mismatch")
	}

	// 3. Decompress & Check Bzip2
	bz2Reader := bzip2.NewReader(bytes.NewReader(res.Bz2.Data))
	bz2Decompressed, err := io.ReadAll(bz2Reader)
	if err != nil {
		t.Fatalf("failed to decompress bzip2: %v", err)
	}
	if !bytes.Equal(bz2Decompressed, testData) {
		t.Errorf("bzip2 decompressed mismatch")
	}

	// 4. Decompress & Check XZ
	xzReader, err := xz.NewReader(bytes.NewReader(res.Xz.Data))
	if err != nil {
		t.Fatalf("failed to read xz: %v", err)
	}
	xzDecompressed, err := io.ReadAll(xzReader)
	if err != nil {
		t.Fatalf("failed to decompress xz: %v", err)
	}
	if !bytes.Equal(xzDecompressed, testData) {
		t.Errorf("xz decompressed mismatch")
	}

	// Verify Checksums are non-empty and have appropriate lengths
	for _, out := range res.AllOutputs() {
		if len(out.MD5) != 32 {
			t.Errorf("%s MD5 len = %d, want 32", out.Ext, len(out.MD5))
		}
		if len(out.SHA1) != 40 {
			t.Errorf("%s SHA1 len = %d, want 40", out.Ext, len(out.SHA1))
		}
		if len(out.SHA256) != 64 {
			t.Errorf("%s SHA256 len = %d, want 64", out.Ext, len(out.SHA256))
		}
		if len(out.SHA512) != 128 {
			t.Errorf("%s SHA512 len = %d, want 128", out.Ext, len(out.SHA512))
		}
	}
}
