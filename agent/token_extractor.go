package agent

import (
	"log"
	"os"
)

const (
	filePath = "/etc/openframe/token.txt"
)

type TokenExtractor struct {
	encryptionService *EncryptionService
}

func NewTokenExtractor(encryptionService *EncryptionService) *TokenExtractor {
	return &TokenExtractor{
		encryptionService: encryptionService,
	}
}

func (te *TokenExtractor) ExtractToken() (string, error) {
	// Read the encrypted token from file
	encryptedData, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Error reading token file: %v", err)
		return "", err
	}

	// Decrypt the data
	decryptedData, err := te.encryptionService.Decrypt(string(encryptedData))
	if err != nil {
		log.Printf("Error decrypting data: %v", err)
		return "", err
	}

	token := string(decryptedData)
	return token, nil
}
