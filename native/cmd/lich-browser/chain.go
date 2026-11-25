package main

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// A server that sends its certificate without the intermediate that signed
// it fails in WebKitGTK and curl but loads in Chrome, which fetches the
// missing issuer from the certificate's Authority Information Access URL.
// completeChain does the same, so a page Chrome opens opens here too.
const (
	// maxIssuerFetches bounds the intermediates fetched for one certificate:
	// public chains carry one or two.
	maxIssuerFetches = 3
	// maxIssuerBytes is far above any real certificate.
	maxIssuerBytes = 64 << 10
	issuerTimeout  = 10 * time.Second
)

var errNoLeaf = errors.New("no certificate in the chain")

// completeChain verifies the PEM chain a server sent for host, fetching the
// issuers it left out, and answers the leaf as PEM once the chain reaches a
// root in roots (the system's when nil). It fails whenever the server sent
// anything else wrong: a name, a date, an untrusted root.
func completeChain(ctx context.Context, client *http.Client, roots *x509.CertPool, host string, chain []byte) ([]byte, error) {
	certs, err := parsePEM(chain)
	if err != nil {
		return nil, err
	}
	leaf := certs[0]
	intermediates := x509.NewCertPool()
	for _, c := range certs[1:] {
		intermediates.AddCert(c)
	}
	opts := x509.VerifyOptions{DNSName: host, Roots: roots, Intermediates: intermediates}
	last := leaf
	for range maxIssuerFetches {
		if len(last.IssuingCertificateURL) == 0 {
			break
		}
		issuer, err := fetchIssuer(ctx, client, last.IssuingCertificateURL[0])
		if err != nil {
			return nil, err
		}
		intermediates.AddCert(issuer)
		if _, err := leaf.Verify(opts); err == nil {
			return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leaf.Raw}), nil
		}
		last = issuer
	}
	_, err = leaf.Verify(opts)
	if err == nil {
		err = errors.New("the chain verifies without fetching")
	}
	return nil, err
}

func parsePEM(chain []byte) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	for {
		var block *pem.Block
		block, chain = pem.Decode(chain)
		if block == nil {
			break
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		certs = append(certs, c)
	}
	if len(certs) == 0 {
		return nil, errNoLeaf
	}
	return certs, nil
}

// fetchIssuer reads a certificate from an AIA URL, which serves DER (RFC
// 5280 4.2.2.1), or PEM from some CAs.
func fetchIssuer(ctx context.Context, client *http.Client, url string) (*x509.Certificate, error) {
	ctx, cancel := context.WithTimeout(ctx, issuerTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("issuer %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxIssuerBytes))
	if err != nil {
		return nil, err
	}
	if block, _ := pem.Decode(body); block != nil {
		body = block.Bytes
	}
	return x509.ParseCertificate(body)
}
