// Copyright 2026 The Atrinik Project
// SPDX-License-Identifier: MIT

package publisher

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"testing"
	"time"
)

func TestParseIdentityPEMRequiresMatchingP256Pair(t *testing.T) {
	t.Parallel()
	certificatePEM, privateKeyPEM := testIdentityPEM(t)
	identity, err := ParseIdentityPEM(certificatePEM, privateKeyPEM)
	if err != nil || len(identity.certificateDER) == 0 || len(identity.serverID) != 64 {
		t.Fatalf("identity = %#v, error = %v", identity, err)
	}
	_, otherKey := testIdentityPEM(t)
	for _, test := range []struct {
		name        string
		certificate []byte
		key         []byte
	}{
		{"missing certificate", nil, privateKeyPEM},
		{"extra certificate block", append(append([]byte(nil), certificatePEM...), certificatePEM...), privateKeyPEM},
		{"wrong key", certificatePEM, otherKey},
		{"wrong key type", certificatePEM, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("bad")})},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := ParseIdentityPEM(test.certificate, test.key); err == nil {
				t.Fatal("invalid identity was accepted")
			}
		})
	}
}

func TestLoadIdentityRequiresOwnerOnlyKeyWithinStateRoot(t *testing.T) {
	t.Parallel()
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	certificate, privateKey := testIdentityPEM(t)
	if err := root.WriteFile("certificate.pem", certificate, 0o644); err != nil ||
		root.WriteFile("private.pem", privateKey, 0o644) != nil {
		t.Fatal("write identity fixture")
	}
	if _, err := LoadIdentity(root, "certificate.pem", "private.pem"); err == nil {
		t.Fatal("permissive private key was accepted")
	}
	if err := root.Chmod("private.pem", 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadIdentity(root, "certificate.pem", "private.pem"); err != nil {
		t.Fatalf("owner-only identity failed: %v", err)
	}
}

func testIdentityPEM(t *testing.T) ([]byte, []byte) {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "publisher test"},
		NotBefore: time.Unix(1, 0), NotAfter: time.Unix(4_000_000_000, 0),
		BasicConstraintsValid: true,
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})
}
