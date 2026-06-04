package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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
	csrfTokenPurpose  = "csrf"
)

var (
	errInvalidSession           = errors.New("invalid session")
	errSessionExpired           = errors.New("session expired")
	errSessionKeyNotInitialized = errors.New("session key not initialized")
	errSecretRequired           = errors.New("secret must not be empty")
)

type SessionData struct {
	Email       string    `json:"email"`
	Name        string    `json:"name"`
	Subject     string    `json:"sub"`
	AccessToken string    `json:"accessToken"`
	Provider    string    `json:"provider"`
	Expiry      time.Time `json:"expiry"`
	CSRFNonce   string    `json:"csrfNonce"`
}

var sessionKey atomic.Pointer[[]byte]

func InitSessionKey(secret string) error {
	if secret == "" {
		return errSecretRequired
	}

	hash := sha256.Sum256([]byte(secret))
	key := hash[:]

	sessionKey.Store(&key)

	return nil
}

func getSessionKey() []byte {
	p := sessionKey.Load()
	if p == nil {
		return nil
	}

	return *p
}

// DeriveCSRFToken produces a CSRF token bound to the session subject and a random nonce
// using HMAC-SHA256 with the session encryption key.
func DeriveCSRFToken(session SessionData) (string, error) {
	key := getSessionKey()
	if key == nil {
		return "", errSessionKeyNotInitialized
	}

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(csrfTokenPurpose))
	mac.Write([]byte(session.Subject))
	mac.Write([]byte(session.CSRFNonce))

	return hex.EncodeToString(mac.Sum(nil)), nil
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

	sealedData := aesGCM.Seal(nonce, nonce, jsonData, nil)

	return base64.URLEncoding.EncodeToString(sealedData), nil
}

func decryptSession(encoded string) (SessionData, error) {
	var data SessionData

	key := getSessionKey()
	if key == nil {
		return data, errSessionKeyNotInitialized
	}

	payload, err := base64.URLEncoding.DecodeString(encoded)
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
	if len(payload) < nonceSize {
		return data, errInvalidSession
	}

	nonce := payload[:nonceSize]
	encryptedData := payload[nonceSize:]

	plaintext, err := aesGCM.Open(nil, nonce, encryptedData, nil)
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
