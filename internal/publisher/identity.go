// Copyright 2026 The Atrinik Project
// SPDX-License-Identifier: MIT

// Package publisher implements the Game Protocol 1 metaserver publisher.
package publisher

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"io"
	"os"
)

const maximumIdentityFileBytes = 16 * 1024

// Identity is the certificate-bound publisher identity. Its fields are kept
// private so callers cannot accidentally log key or certificate material.
type Identity struct {
	certificateDER []byte
	privateKey     *ecdsa.PrivateKey
	serverID       string
}

// LoadIdentity reads one PEM leaf certificate and matching P-256 private key.
// Certificate wall-clock validity is deliberately not an identity input.
func LoadIdentity(root *os.Root, certificatePath, privateKeyPath string) (*Identity, error) {
	if root == nil {
		return nil, errors.New("publisher state root is nil")
	}
	certificatePEM, err := readBoundedFile(root, certificatePath, false)
	if err != nil {
		return nil, errors.New("load publisher certificate")
	}
	privateKeyPEM, err := readBoundedFile(root, privateKeyPath, true)
	if err != nil {
		return nil, errors.New("load publisher private key")
	}
	identity, err := ParseIdentityPEM(certificatePEM, privateKeyPEM)
	clear(privateKeyPEM)
	if err != nil {
		return nil, err
	}
	return identity, nil
}

// ParseIdentityPEM validates one certificate/key pair. It is exported for
// deterministic integration with the future QUIC identity owner.
func ParseIdentityPEM(certificatePEM, privateKeyPEM []byte) (*Identity, error) {
	certificateBlock, certificateRest := pem.Decode(certificatePEM)
	if certificateBlock == nil || certificateBlock.Type != "CERTIFICATE" ||
		len(bytes.TrimSpace(certificateRest)) != 0 {
		return nil, errors.New("publisher certificate must contain one PEM certificate")
	}
	certificate, err := x509.ParseCertificate(certificateBlock.Bytes)
	if err != nil {
		return nil, errors.New("publisher certificate is invalid")
	}
	publicKey, ok := certificate.PublicKey.(*ecdsa.PublicKey)
	if !ok || publicKey.Curve != elliptic.P256() {
		return nil, errors.New("publisher certificate must use ECDSA P-256")
	}

	privateBlock, privateRest := pem.Decode(privateKeyPEM)
	if privateBlock == nil || len(bytes.TrimSpace(privateRest)) != 0 {
		return nil, errors.New("publisher private key must contain one PEM key")
	}
	privateKey, err := parsePrivateKey(privateBlock)
	if err != nil || privateKey.Curve != elliptic.P256() {
		return nil, errors.New("publisher private key does not match the P-256 certificate")
	}
	privatePublicBytes, privatePublicErr := privateKey.PublicKey.Bytes()
	certificatePublicBytes, certificatePublicErr := publicKey.Bytes()
	if privatePublicErr != nil || certificatePublicErr != nil ||
		!bytes.Equal(privatePublicBytes, certificatePublicBytes) {
		return nil, errors.New("publisher private key does not match the P-256 certificate")
	}
	fingerprint := sha256.Sum256(certificateBlock.Bytes)
	return &Identity{
		certificateDER: append([]byte(nil), certificateBlock.Bytes...),
		privateKey:     privateKey,
		serverID:       hex.EncodeToString(fingerprint[:]),
	}, nil
}

func parsePrivateKey(block *pem.Block) (*ecdsa.PrivateKey, error) {
	switch block.Type {
	case "EC PRIVATE KEY":
		return x509.ParseECPrivateKey(block.Bytes)
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		privateKey, ok := key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, errors.New("private key is not ECDSA")
		}
		return privateKey, nil
	default:
		return nil, errors.New("unsupported private key PEM type")
	}
}

func readBoundedFile(root *os.Root, path string, requireOwnerOnly bool) ([]byte, error) {
	file, err := root.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	information, err := file.Stat()
	if err != nil || !information.Mode().IsRegular() || information.Size() < 1 ||
		information.Size() > maximumIdentityFileBytes {
		return nil, errors.New("identity file is outside supported bounds")
	}
	if requireOwnerOnly && validateOwnerOnlyFile(file) != nil {
		return nil, errors.New("identity file is outside supported bounds")
	}
	data := make([]byte, information.Size())
	if _, err := io.ReadFull(file, data); err != nil {
		clear(data)
		return nil, err
	}
	return data, nil
}
