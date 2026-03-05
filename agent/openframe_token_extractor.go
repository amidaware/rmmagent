package agent

import (
	"os"
	"github.com/sirupsen/logrus"
)

type OpenframeTokenExtractor struct {
	encryptionService *OpenframeEncryptionService
	filePath          string
	logger            *logrus.Logger
	readErrCount      int
}

func NewOpenframeTokenExtractor(encryptionService *OpenframeEncryptionService, filePath string, logger *logrus.Logger) *OpenframeTokenExtractor {
	return &OpenframeTokenExtractor{
		encryptionService: encryptionService,
		filePath:          filePath,
		logger:            logger,
	}
}

func (te *OpenframeTokenExtractor) ExtractToken() (string, error) {
	encryptedData, err := os.ReadFile(te.filePath)
	if err != nil {
		te.readErrCount++
		if te.readErrCount % openframeTokenRefreshErrorLogInterval == 1 {
			te.logger.Errorf("Error reading token file: %v", err)
		}
		return "", err
	}
	te.readErrCount = 0

	decryptedData, err := te.encryptionService.Decrypt(string(encryptedData))
	if err != nil {
		te.logger.Errorf("Error decrypting data: %v", err)
		return "", err
	}

	token := string(decryptedData)
	return token, nil
}
