package services

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
)

// generateSelfSignedCert returns a *tls.Certificate backed by a fresh ECDSA
// P-256 key. The certificate is valid for the loopback address and "localhost".
func generateSelfSignedCert(t testing.TB) tls.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generateSelfSignedCert: key generation failed: %v", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("generateSelfSignedCert: certificate creation failed: %v", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("generateSelfSignedCert: key marshal failed: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("generateSelfSignedCert: tls.X509KeyPair failed: %v", err)
	}

	// Attach the parsed leaf so the caller can build a CertPool from it.
	cert.Leaf, err = x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("generateSelfSignedCert: leaf parse failed: %v", err)
	}

	return cert
}

// StartSCPWithTLS starts a TLS-enabled SCP on port and returns a cleanup func
// along with the SCP instance. It also returns the *x509.CertPool that clients
// should use to trust the server's self-signed certificate.
func StartSCPWithTLS(t testing.TB, port int) (func(testing.TB), SCP, *x509.CertPool) {
	t.Helper()

	cert := generateSelfSignedCert(t)

	serverTLS := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	pool := x509.NewCertPool()
	pool.AddCert(cert.Leaf)

	scpInst := NewSCPWithTLS(port, serverTLS)
	go func() {
		if err := scpInst.Start(context.Background()); err != nil {
			t.Logf("TLS SCP stopped: %v", err)
		}
	}()

	// Poll until the port is accepting TLS connections.
	addr := fmt.Sprintf("localhost:%d", port)
	deadline := time.Now().Add(2 * time.Second)
	clientTLS := &tls.Config{RootCAs: pool, ServerName: "localhost"}
	for time.Now().Before(deadline) {
		conn, err := tls.Dial("tcp", addr, clientTLS)
		if err == nil {
			conn.Close()
			time.Sleep(20 * time.Millisecond)
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	cleanup := func(t testing.TB) {
		if err := scpInst.Stop(); err != nil {
			t.Errorf("failed to stop TLS SCP: %v", err)
		}
	}
	t.Cleanup(func() { cleanup(t) })
	return cleanup, scpInst, pool
}

func Test_scu_EchoSCU_TLS(t *testing.T) {
	const tlsPort = 1041

	_, testSCP, pool := StartSCPWithTLS(t, tlsPort)

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return request.GetCalledAE() == "TLS_SCP"
	})

	media.InitDict()

	clientTLS := &tls.Config{
		RootCAs:    pool,
		ServerName: "localhost",
		MinVersion: tls.VersionTLS12,
	}

	dest := &network.Destination{
		Name:      "TLS Test",
		CalledAE:  "TLS_SCP",
		CallingAE: "TLS_SCU",
		HostName:  "localhost",
		Port:      tlsPort,
		IsTLS:     true,
		TLSConfig: clientTLS,
	}

	scu := NewSCU(dest)
	if err := scu.EchoSCU(context.Background(), 10); err != nil {
		t.Fatalf("TLS C-Echo failed: %v", err)
	}
}

func Test_scu_EchoSCU_TLS_wrongCA(t *testing.T) {
	const tlsPort = 1042

	_, testSCP, _ := StartSCPWithTLS(t, tlsPort)

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return true
	})

	media.InitDict()

	// Intentionally use a *different* cert pool — the client should reject the server cert.
	differentCert := generateSelfSignedCert(t)
	wrongPool := x509.NewCertPool()
	wrongPool.AddCert(differentCert.Leaf)

	dest := &network.Destination{
		Name:      "TLS Wrong CA",
		CalledAE:  "TLS_SCP",
		CallingAE: "TLS_SCU",
		HostName:  "localhost",
		Port:      tlsPort,
		IsTLS:     true,
		TLSConfig: &tls.Config{
			RootCAs:    wrongPool,
			ServerName: "localhost",
			MinVersion: tls.VersionTLS12,
		},
	}

	scu := NewSCU(dest)
	err := scu.EchoSCU(context.Background(), 10)
	if err == nil {
		t.Fatal("expected TLS handshake to fail with wrong CA, but it succeeded")
	}
}

func Test_scu_EchoSCU_TLSPort_PlainClient(t *testing.T) {
	const tlsPort = 1043

	_, testSCP, _ := StartSCPWithTLS(t, tlsPort)

	testSCP.OnAssociationRequest(func(request network.AssociationRequest) bool {
		return true
	})

	media.InitDict()

	// Plain (non-TLS) SCU connecting to a TLS SCP must fail.
	dest := &network.Destination{
		Name:      "Plain to TLS",
		CalledAE:  "TLS_SCP",
		CallingAE: "PLAIN_SCU",
		HostName:  "localhost",
		Port:      tlsPort,
		IsTLS:     false,
	}

	scu := NewSCU(dest)
	err := scu.EchoSCU(context.Background(), 5)
	if err == nil {
		t.Fatal("expected plain SCU to fail against TLS SCP, but it succeeded")
	}
}

func TestNewSCPWithTLS_EnforcesMinimumTLS12(t *testing.T) {
	s := NewSCPWithTLS(11111, &tls.Config{MinVersion: tls.VersionTLS10}).(*scp)
	if s.tlsConfig == nil {
		t.Fatal("tlsConfig is nil")
	}
	if s.tlsConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %d, want %d", s.tlsConfig.MinVersion, tls.VersionTLS12)
	}
}

func TestNewSCPWithTLS_DefaultConfigUsesTLS12(t *testing.T) {
	s := NewSCPWithTLS(11112, nil).(*scp)
	if s.tlsConfig == nil {
		t.Fatal("tlsConfig is nil")
	}
	if s.tlsConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %d, want %d", s.tlsConfig.MinVersion, tls.VersionTLS12)
	}
}
