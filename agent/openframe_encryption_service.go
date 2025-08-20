package agent

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"log"
)

type OpenframeEncryptionService struct {
	encryptionKey string
}

func NewOpenframeEncryptionService(encryptionKey string) *OpenframeEncryptionService {
	return &OpenframeEncryptionService{
		encryptionKey: encryptionKey,
	}
}


func (es *OpenframeEncryptionService) Decrypt(data string) ([]byte, error) {
	// Decode base64 string to bytes
	encryptedData, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		log.Printf("Error decoding base64 data: %v", err)
		return nil, err
	}

	block, err := aes.NewCipher([]byte(es.encryptionKey))
	if err != nil {
		log.Printf("Error creating cipher: %v", err)
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(encryptedData) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce := encryptedData[:gcm.NonceSize()]
	ciphertext := encryptedData[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
