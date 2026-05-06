package network

import "crypto/tls"

// Destination - a DICOM destination
type Destination struct {
	ID        string
	Name      string
	HostName  string
	CalledAE  string
	CallingAE string
	Port      int
	// IsTLS enables TLS for outbound connections. TLSConfig must also be set.
	IsTLS bool
	// TLSConfig is the TLS configuration used when IsTLS is true. nil causes
	// the system certificate pool to be used with no client certificate.
	TLSConfig *tls.Config
}
