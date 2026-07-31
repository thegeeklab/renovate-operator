package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TLSConfig", func() {
	var (
		validCACert []byte
		serverCert  *x509.Certificate
	)

	BeforeEach(func() {
		caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		Expect(err).NotTo(HaveOccurred())

		caTmpl := &x509.Certificate{
			SerialNumber:          big.NewInt(1),
			Subject:               pkix.Name{CommonName: "test-ca"},
			NotBefore:             time.Now().Add(-time.Hour),
			NotAfter:              time.Now().Add(time.Hour),
			IsCA:                  true,
			BasicConstraintsValid: true,
			KeyUsage:              x509.KeyUsageCertSign,
		}

		caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
		Expect(err).NotTo(HaveOccurred())

		validCACert = pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: caDER,
		})

		serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		Expect(err).NotTo(HaveOccurred())

		serverTmpl := &x509.Certificate{
			SerialNumber: big.NewInt(2),
			Subject:      pkix.Name{CommonName: "test-server"},
			DNSNames:     []string{"test-server"},
			NotBefore:    time.Now().Add(-time.Hour),
			NotAfter:     time.Now().Add(time.Hour),
		}

		serverDER, err := x509.CreateCertificate(rand.Reader, serverTmpl, caTmpl, &serverKey.PublicKey, caKey)
		Expect(err).NotTo(HaveOccurred())

		serverCert, err = x509.ParseCertificate(serverDER)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("NewTLSConfig", func() {
		It("should return nil for a default config", func() {
			cfg := NewTLSConfig(false, nil)
			Expect(cfg).To(BeNil())
		})

		It("should return nil for empty caCert", func() {
			cfg := NewTLSConfig(false, []byte{})
			Expect(cfg).To(BeNil())
		})

		It("should set InsecureSkipVerify when insecure is true", func() {
			cfg := NewTLSConfig(true, nil)
			Expect(cfg).NotTo(BeNil())
			Expect(cfg.InsecureSkipVerify).To(BeTrue())
		})

		It("should prefer CA pool over InsecureSkipVerify when both are set", func() {
			cfg := NewTLSConfig(true, validCACert)
			Expect(cfg).NotTo(BeNil())
			Expect(cfg.InsecureSkipVerify).To(BeFalse())
			Expect(cfg.RootCAs).NotTo(BeNil())
		})

		It("should return a config with system cert pool plus custom CA", func() {
			cfg := NewTLSConfig(false, validCACert)
			Expect(cfg).NotTo(BeNil())
			Expect(cfg.InsecureSkipVerify).To(BeFalse())
			Expect(cfg.RootCAs).NotTo(BeNil())
		})

		It("should produce a pool that verifies a certificate signed by the custom CA", func() {
			cfg := NewTLSConfig(false, validCACert)
			Expect(cfg).NotTo(BeNil())

			verifiedChains, err := serverCert.Verify(x509.VerifyOptions{
				Roots: cfg.RootCAs,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(verifiedChains).NotTo(BeEmpty())
		})
	})
})
