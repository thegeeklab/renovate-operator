package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

const (
	sessionCookieName = "renovate_session"
	sessionDuration   = 24 * time.Hour
)

var (
	errInvalidSession           = errors.New("invalid session")
	errSessionExpired           = errors.New("session expired")
	errSessionKeyNotInitialized = errors.New("session key not initialized")
)

type SessionData struct {
	Email       string    `json:"email"`
	Name        string    `json:"name"`
	Subject     string    `json:"sub"`
	AccessToken string    `json:"accessToken"`
	Provider    string    `json:"provider"`
	Expiry      time.Time `json:"expiry"`
}

var sessionKey atomic.Pointer[[]byte]

func InitSessionKey(secret string) {
	var key []byte
	if secret == "" {
		key = make([]byte, 32) //nolint:mnd
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			panic(fmt.Sprintf("failed to generate session key: %v", err))
		}
	} else {
		hash := sha256.Sum256([]byte(secret))
		key = hash[:]
	}

	sessionKey.Store(&key)
}

func getSessionKey() []byte {
	p := sessionKey.Load()
	if p == nil {
		return nil
	}

	return *p
}

func encryptSession(data SessionData) (string, error) {
	key := getSessionKey()
	if key == nil {
		return "", errSessionKeyNotInitialized
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal session: %w", err)
	}

	ciphertext := aesGCM.Seal(nonce, nonce, jsonData, nil)

	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

func decryptSession(encoded string) (SessionData, error) {
	var data SessionData

	key := getSessionKey()
	if key == nil {
		return data, errSessionKeyNotInitialized
	}

	ciphertext, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return data, fmt.Errorf("%w: invalid encoding", errInvalidSession)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return data, fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return data, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return data, errInvalidSession
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return data, fmt.Errorf("%w: decryption failed", errInvalidSession)
	}

	if err := json.Unmarshal(plaintext, &data); err != nil {
		return data, fmt.Errorf("%w: invalid JSON", errInvalidSession)
	}

	if time.Now().After(data.Expiry) {
		return data, errSessionExpired
	}

	return data, nil
}
