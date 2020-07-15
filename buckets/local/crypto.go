package local

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/textileio/dcrypto"
)

// EncryptLocalPath encrypts the file at path with password, writing the result to the writer.
// Encryption is AES-CTR + AES-512 HMAC.
// https://godoc.org/golang.org/x/crypto/scrypt is used to derive the keys from the password.
func (b *Bucket) EncryptLocalPath(pth, password string, w io.Writer) error {
	file, err := os.Open(pth)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("path %s is not a file", pth)
	}
	r, err := dcrypto.NewEncrypterWithPassword(file, []byte(password))
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, r); err != nil {
		return err
	}
	return nil
}

// DecryptLocalPath decrypts the file at path with password, writing the result to the writer.
// Encryption is AES-CTR + AES-512 HMAC.
// https://godoc.org/golang.org/x/crypto/scrypt is used to derive the keys from the password.
func (b *Bucket) DecryptLocalPath(pth, password string, w io.Writer) error {
	file, err := os.Open(pth)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("path %s is not a file", pth)
	}
	r, err := dcrypto.NewDecrypterWithPassword(file, []byte(password))
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, r); err != nil {
		return err
	}
	return nil
}

// DecryptRemotePath decrypts the file at the remote path with password, writing the result to the writer.
// Encryption is AES-CTR + AES-512 HMAC.
// https://godoc.org/golang.org/x/crypto/scrypt is used to derive the keys from the password.
func (b *Bucket) DecryptRemotePath(ctx context.Context, pth, password string, w io.Writer) error {
	ctx, err := b.context(ctx)
	if err != nil {
		return err
	}
	errs := make(chan error)
	reader, writer := io.Pipe()
	go func() {
		defer writer.Close()
		if err := b.clients.Buckets.PullPath(ctx, b.Key(), pth, writer); err != nil {
			errs <- err
		}
	}()
	go func() {
		defer close(errs)
		r, err := dcrypto.NewDecrypterWithPassword(reader, []byte(password))
		if err != nil {
			errs <- err
			return
		}
		defer r.Close()
		if _, err := io.Copy(w, r); err != nil {
			errs <- err
			return
		}
	}()
	return <-errs
}
