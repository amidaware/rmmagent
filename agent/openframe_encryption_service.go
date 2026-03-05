package agent

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"github.com/sirupsen/logrus"
)

type OpenframeEncryptionService struct {
	encryptionKey   string
	logger          *logrus.Logger
	decryptErrCount int
}

func NewOpenframeEncryptionService(encryptionKey string, logger *logrus.Logger) *OpenframeEncryptionService {
	return &OpenframeEncryptionService{
		encryptionKey: encryptionKey,
		logger:        logger,
	}
}

func (es *OpenframeEncryptionService) Decrypt(data string) ([]byte, error) {
	encryptedData, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		es.decryptErrCount++
		if es.decryptErrCount%openframeTokenRefreshErrorLogInterval == 1 {
			es.logger.Errorf("Error decoding base64 data: %v", err)
		}
		return nil, err
	}

	block, err := aes.NewCipher([]byte(es.encryptionKey))
	if err != nil {
		es.decryptErrCount++
		if es.decryptErrCount % openframeTokenRefreshErrorLogInterval == 1 {
			es.logger.Errorf("Error creating cipher: %v", err)
		}
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(encryptedData) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}

	// Extract nonce from the beginning of encrypted data
	nonce := encryptedData[:gcm.NonceSize()]
	ciphertext := encryptedData[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		es.decryptErrCount++
		if es.decryptErrCount % openframeTokenRefreshErrorLogInterval == 1 {
			es.logger.Errorf("Error decrypting data: %v", err)
		}
		return nil, err
	}
	es.decryptErrCount = 0

	return plaintext, nil
}
