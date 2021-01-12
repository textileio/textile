package local

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/textileio/dcrypto"
)

// EncryptLocalPath encrypts the file at path with key, writing the result to the writer.
// Encryption is AES-CTR + AES-512 HMAC.
func (b *Bucket) EncryptLocalPath(pth string, key []byte, w io.Writer) error {
	return encryptLocalPath(pth, key, dcrypto.NewEncrypter, w)
}

// DecryptLocalPath decrypts the file at path with key, writing the result to the writer.
// Encryption is AES-CTR + AES-512 HMAC.
func (b *Bucket) DecryptLocalPath(pth string, key []byte, w io.Writer) error {
	return decryptLocalPath(pth, key, dcrypto.NewDecrypter, w)
}

// EncryptLocalPathWithPassword encrypts the file at path with password, writing the result to the writer.
// Encryption is AES-CTR + AES-512 HMAC.
// https://godoc.org/golang.org/x/crypto/scrypt is used to derive the keys from the password.
func (b *Bucket) EncryptLocalPathWithPassword(pth, password string, w io.Writer) error {
	return encryptLocalPath(pth, []byte(password), dcrypto.NewEncrypterWithPassword, w)
}

// DecryptLocalPathWithPassword decrypts the file at path with password, writing the result to the writer.
// Encryption is AES-CTR + AES-512 HMAC.
// https://godoc.org/golang.org/x/crypto/scrypt is used to derive the keys from the password.
func (b *Bucket) DecryptLocalPathWithPassword(pth, password string, w io.Writer) error {
	return decryptLocalPath(pth, []byte(password), dcrypto.NewDecrypterWithPassword, w)
}

type encryptFunc func(io.Reader, []byte) (io.Reader, error)
type decryptFunc func(io.Reader, []byte) (io.ReadCloser, error)

func encryptLocalPath(pth string, keyOrPassword []byte, fn encryptFunc, w io.Writer) error {
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

	r, err := fn(file, keyOrPassword)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, r); err != nil {
		return err
	}
	return nil
}

func decryptLocalPath(pth string, keyOrPassword []byte, fn decryptFunc, w io.Writer) error {
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
	r, err := fn(file, keyOrPassword)
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
func (b *Bucket) DecryptRemotePath(ctx context.Context, pth string, key []byte, w io.Writer) error {
	return b.decryptRemotePath(ctx, pth, key, dcrypto.NewDecrypter, w)
}

// DecryptRemotePathWithPassword decrypts the file at the remote path with password, writing the result to the writer.
// Encryption is AES-CTR + AES-512 HMAC.
// https://godoc.org/golang.org/x/crypto/scrypt is used to derive the keys from the password.
func (b *Bucket) DecryptRemotePathWithPassword(ctx context.Context, pth, password string, w io.Writer) error {
	return b.decryptRemotePath(ctx, pth, []byte(password), dcrypto.NewDecrypterWithPassword, w)
}

func (b *Bucket) decryptRemotePath(
	ctx context.Context,
	pth string,
	keyOrPassword []byte,
	fn decryptFunc,
	w io.Writer,
) error {
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
		r, err := fn(reader, keyOrPassword)
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
