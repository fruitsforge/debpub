package debian

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

// Signer signs Debian Release manifests.
type Signer interface {
	// SignDetached generates detached signature (Release.gpg)
	SignDetached(ctx context.Context, data []byte) ([]byte, error)
	// SignClear generates inline clearsigned manifest (InRelease)
	SignClear(ctx context.Context, data []byte) ([]byte, error)
}

// GPGSigner implements signing using the local system `gpg` command line binary.
type GPGSigner struct {
	KeyID      string
	Passphrase string
	ExtraArgs  []string
}

// NewGPGSigner creates a GPGSigner with key ID, optional passphrase, and optional extra gpg flags.
func NewGPGSigner(keyID, passphrase string, extraArgs []string) *GPGSigner {
	return &GPGSigner{
		KeyID:      keyID,
		Passphrase: passphrase,
		ExtraArgs:  extraArgs,
	}
}

// SignDetached executes `gpg --armor --detach-sign` to produce Release.gpg
func (s *GPGSigner) SignDetached(ctx context.Context, data []byte) ([]byte, error) {
	args := []string{"--batch", "--no-tty", "--armor", "--detach-sign"}
	if s.KeyID != "" {
		args = append(args, "--local-user", s.KeyID)
	}
	args = append(args, s.ExtraArgs...)

	return s.runGPG(ctx, args, data)
}

// SignClear executes `gpg --armor --clearsign` to produce InRelease
func (s *GPGSigner) SignClear(ctx context.Context, data []byte) ([]byte, error) {
	args := []string{"--batch", "--no-tty", "--armor", "--clearsign"}
	if s.KeyID != "" {
		args = append(args, "--local-user", s.KeyID)
	}
	args = append(args, s.ExtraArgs...)

	return s.runGPG(ctx, args, data)
}

func (s *GPGSigner) runGPG(ctx context.Context, args []string, input []byte) ([]byte, error) {
	cmdArgs := append([]string(nil), args...)

	var passReader, passWriter *os.File
	if s.Passphrase != "" {
		var err error
		passReader, passWriter, err = os.Pipe()
		if err != nil {
			return nil, fmt.Errorf("gpg passphrase pipe creation failed: %w", err)
		}
		defer passReader.Close()

		cmdArgs = append(cmdArgs, "--pinentry-mode", "loopback", "--passphrase-fd", "3")
	}

	cmd := exec.CommandContext(ctx, "gpg", cmdArgs...)
	cmd.Stdin = bytes.NewReader(input)
	if passReader != nil {
		cmd.ExtraFiles = []*os.File{passReader}
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if passWriter != nil {
		go func() {
			defer passWriter.Close()
			_, _ = passWriter.WriteString(s.Passphrase + "\n")
		}()
	}

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gpg execution failed (%s): %w", stringsTrim(stderr.String()), err)
	}

	return stdout.Bytes(), nil
}

func stringsTrim(s string) string {
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
