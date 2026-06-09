package cryptox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	"github.com/pulsoats/core/errorsx"
)

type Encryptor struct {
	key []byte
}

func NewEncryptor(key []byte) *Encryptor {
	return &Encryptor{key: key}
}

func (e *Encryptor) Encrypt(plaintext string) ([]byte, error) {
	const op = "encrypt"
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("%s: new cipher: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%s: new gcm: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("%s: read nonce: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	return gcm.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

func (e *Encryptor) Decrypt(ciphertext []byte) (string, error) {
	const op = "decrypt"
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("%s: new cipher: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("%s: new gcm: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("%s: ciphertext too short", op)
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	return string(plaintext), nil
}
