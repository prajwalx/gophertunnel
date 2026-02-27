package security

import (
	"crypto/aes"
	"crypto/cipher"
)

// NewStream creates a CTR stream for encryption/decryption
// In a real app, use a PBKDF2 hash of a passcode to generate this 32-byte key
var SharedKey = []byte("12345678901234567890123456789012")
var FixedIV = []byte("1234567890123456") // In production, send a random IV in the header

func GetStream(key []byte) (cipher.Stream, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewCTR(block, FixedIV), nil

}
