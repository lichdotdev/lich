package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func issue(t *testing.T, tmpl *x509.Certificate, parent *testCA) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl.SerialNumber = big.NewInt(time.Now().UnixNano())
	tmpl.NotBefore = time.Now().Add(-time.Hour)
	if tmpl.NotAfter.IsZero() {
		tmpl.NotAfter = time.Now().Add(time.Hour)
	}
	signer, signerKey := tmpl, key
	if parent != nil {
		signer, signerKey = parent.cert, parent.key
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, signer, &key.PublicKey, signerKey)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return &testCA{cert: cert, key: key}
}

func caTemplate(name string) *x509.Certificate {
	return &x509.Certificate{
		Subject: pkix.Name{CommonName: name}, IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign,
	}
}

func pemOf(certs ...*x509.Certificate) []byte {
	var out []byte
	for _, c := range certs {
		out = append(out, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})...)
	}
	return out
}

// incompleteServer is a server sending only its leaf, as
// homol.devcenter.sankhya.com.br did, with the intermediate at its AIA URL.
func incompleteServer(t *testing.T, leafTmpl *x509.Certificate) (*x509.CertPool, []byte, *httptest.Server) {
	t.Helper()
	root := issue(t, caTemplate("root"), nil)
	mid := issue(t, caTemplate("intermediate"), root)
	aia := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(mid.cert.Raw)
	}))
	t.Cleanup(aia.Close)
	leafTmpl.IssuingCertificateURL = []string{aia.URL + "/mid.der"}
	leaf := issue(t, leafTmpl, mid)
	roots := x509.NewCertPool()
	roots.AddCert(root.cert)
	return roots, pemOf(leaf.cert), aia
}

func TestCompleteChainFetchesMissingIntermediate(t *testing.T) {
	roots, chain, aia := incompleteServer(t, &x509.Certificate{DNSNames: []string{"*.example.test"}})
	leaf, err := completeChain(context.Background(), aia.Client(), roots, "homol.example.test", chain)
	if err != nil {
		t.Fatal(err)
	}
	if string(leaf) != string(chain) {
		t.Fatalf("leaf = %q, want the server's leaf", leaf)
	}
}

func TestCompleteChainRefusesOtherFaults(t *testing.T) {
	cases := map[string]struct {
		leaf *x509.Certificate
		host string
	}{
		"wrong name": {&x509.Certificate{DNSNames: []string{"example.test"}}, "evil.test"},
		"expired":    {&x509.Certificate{DNSNames: []string{"example.test"}, NotAfter: time.Now().Add(-time.Minute)}, "example.test"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			roots, chain, aia := incompleteServer(t, c.leaf)
			if _, err := completeChain(context.Background(), aia.Client(), roots, c.host, chain); err == nil {
				t.Fatal("completeChain allowed a certificate with another fault")
			}
		})
	}
}

func TestCompleteChainRefusesUntrustedRoot(t *testing.T) {
	_, chain, aia := incompleteServer(t, &x509.Certificate{DNSNames: []string{"example.test"}})
	if _, err := completeChain(context.Background(), aia.Client(), x509.NewCertPool(), "example.test", chain); err == nil {
		t.Fatal("completeChain allowed a chain to a root the system does not trust")
	}
}

func TestCompleteChainRefusesNoCertificate(t *testing.T) {
	if _, err := completeChain(context.Background(), http.DefaultClient, nil, "example.test", []byte("junk")); err == nil {
		t.Fatal("completeChain allowed an empty chain")
	}
}
