package agent

import (
	"log"
	"os"
)

type OpenframeTokenExtractor struct {
	encryptionService *OpenframeEncryptionService
	filePath          string
}

func NewOpenframeTokenExtractor(encryptionService *OpenframeEncryptionService, filePath string) *OpenframeTokenExtractor {
	return &OpenframeTokenExtractor{
		encryptionService: encryptionService,
		filePath:          filePath,
	}
}

func (te *OpenframeTokenExtractor) ExtractToken() (string, error) {
	// Read the encrypted token from file
	encryptedData, err := os.ReadFile(te.filePath)
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
