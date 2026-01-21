// Package sshkey provides SSH key generation utilities.
package sshkey

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"golang.org/x/crypto/ssh"
)

// KeyPair holds a public/private SSH key pair.
type KeyPair struct {
	PublicKey  string
	PrivateKey string
}

// Generator generates SSH key pairs.
type Generator interface {
	Generate() (*KeyPair, error)
	GetPublicKey(privateKey string) (*KeyPair, error)
}

type rsaGenerator struct {
	bits int
}

// NewGenerator creates a new SSH key generator with default 4096-bit RSA keys.
func NewGenerator() Generator {
	return &rsaGenerator{bits: 4096}
}

// NewGeneratorWithBits creates a new SSH key generator with specified RSA key size.
func NewGeneratorWithBits(bits int) Generator {
	return &rsaGenerator{bits: bits}
}

func (g *rsaGenerator) Generate() (*KeyPair, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, g.bits)
	if err != nil {
		return nil, err
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, err
	}

	return &KeyPair{
		PrivateKey: string(privateKeyPEM),
		PublicKey:  string(ssh.MarshalAuthorizedKey(publicKey)),
	}, nil
}

func (g *rsaGenerator) GetPublicKey(privateKeyPEM string) (*KeyPair, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return &KeyPair{PrivateKey: privateKeyPEM}, nil
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return &KeyPair{PrivateKey: privateKeyPEM}, nil
	}

	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return &KeyPair{PrivateKey: privateKeyPEM}, nil
	}

	return &KeyPair{
		PrivateKey: privateKeyPEM,
		PublicKey:  string(ssh.MarshalAuthorizedKey(publicKey)),
	}, nil
}
