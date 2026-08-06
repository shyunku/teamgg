package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"
)

func MimicOpenSSLGenRSA() []byte {
	rawPkey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}

	return pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY",
		// Headers: map[string]string{},
		Bytes: x509.MarshalPKCS1PrivateKey(rawPkey),
	})
}

func MimicOpenSSLReqNew(pemPkey []byte) []byte {
	pemPkeyBlock, _ := pem.Decode(pemPkey)
	pkey, err := x509.ParsePKCS1PrivateKey(pemPkeyBlock.Bytes)
	if err != nil {
		panic(err)
	}
	cert := &x509.Certificate{
		Subject: pkix.Name{
			Country:    []string{"KR"},
			CommonName: "Hello",
		},
		SerialNumber: big.NewInt(0),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(100 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	rawCert, err := x509.CreateCertificate(rand.Reader, cert, cert, &pkey.PublicKey, pkey)
	if err != nil {
		panic(err)
	}
	return pem.EncodeToMemory(&pem.Block{
		Type: "CERTIFICATE",
		// Headers: map[string]string{},
		Bytes: rawCert,
	})
}
