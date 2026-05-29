package auth

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Session", func() {
	BeforeEach(func() {
		err := InitSessionKey("test-secret-for-session-encryption")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("InitSessionKey", func() {
		It("should initialize session key from secret", func() {
			err := InitSessionKey("another-secret")
			Expect(err).NotTo(HaveOccurred())
			Expect(getSessionKey()).NotTo(BeEmpty())
			Expect(getSessionKey()).To(HaveLen(32))
		})

		It("should return error when secret is empty", func() {
			err := InitSessionKey("")
			Expect(err).To(MatchError(errSecretRequired))
		})

		It("should produce consistent keys for the same secret", func() {
			err := InitSessionKey("consistent-secret")
			Expect(err).NotTo(HaveOccurred())

			current := getSessionKey()
			key1 := make([]byte, len(current))
			copy(key1, current)

			err = InitSessionKey("consistent-secret")
			Expect(err).NotTo(HaveOccurred())

			current = getSessionKey()
			key2 := make([]byte, len(current))
			copy(key2, current)

			Expect(key1).To(Equal(key2))
		})
	})

	Describe("encryptSession/decryptSession", func() {
		It("should encrypt and decrypt session data", func() {
			original := SessionData{
				Email:       "test@example.com",
				Name:        "Test User",
				Subject:     "sub-123",
				AccessToken: "access-token-123",
				Provider:    "gitea-prod",
				Expiry:      time.Now().Add(time.Hour),
			}

			encrypted, err := encryptSession(original)
			Expect(err).NotTo(HaveOccurred())
			Expect(encrypted).NotTo(BeEmpty())

			decrypted, err := decryptSession(encrypted)
			Expect(err).NotTo(HaveOccurred())
			Expect(decrypted.Email).To(Equal(original.Email))
			Expect(decrypted.Name).To(Equal(original.Name))
			Expect(decrypted.Subject).To(Equal(original.Subject))
			Expect(decrypted.AccessToken).To(Equal(original.AccessToken))
			Expect(decrypted.Provider).To(Equal(original.Provider))
		})

		It("should return error for invalid encrypted data", func() {
			_, err := decryptSession("invalid-base64-data!!!")
			Expect(err).To(HaveOccurred())
		})

		It("should return error for tampered data", func() {
			original := SessionData{
				Email:    "test@example.com",
				Expiry:   time.Now().Add(time.Hour),
				Provider: "gitea-prod",
			}

			encrypted, err := encryptSession(original)
			Expect(err).NotTo(HaveOccurred())

			tampered := encrypted + "tampered"
			_, err = decryptSession(tampered)
			Expect(err).To(HaveOccurred())
		})

		It("should return error for expired session", func() {
			original := SessionData{
				Email:    "test@example.com",
				Expiry:   time.Now().Add(-time.Hour),
				Provider: "gitea-prod",
			}

			encrypted, err := encryptSession(original)
			Expect(err).NotTo(HaveOccurred())

			_, err = decryptSession(encrypted)
			Expect(err).To(MatchError(errSessionExpired))
		})

		It("should produce different ciphertext for same plaintext", func() {
			original := SessionData{
				Email:    "test@example.com",
				Expiry:   time.Now().Add(time.Hour),
				Provider: "gitea-prod",
			}

			enc1, err := encryptSession(original)
			Expect(err).NotTo(HaveOccurred())

			enc2, err := encryptSession(original)
			Expect(err).NotTo(HaveOccurred())

			Expect(enc1).NotTo(Equal(enc2))
		})
	})

	Context("when session key is not initialized", func() {
		BeforeEach(func() {
			sessionKey.Store(nil)
		})

		It("should return error from encryptSession mentioning session key", func() {
			_, err := encryptSession(SessionData{
				Email:    "test@example.com",
				Expiry:   time.Now().Add(time.Hour),
				Provider: "gitea-prod",
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("session key"))
		})
	})
})
