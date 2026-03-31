# Innovative IO Dicom

Main CLI binary for SCU/SCP DICOM network operations and file utilities.
The server (`-scp`) responds to SIGINT/SIGTERM for graceful shutdown.

## Usage

  -calledae string
    	AE title expected by the destination (default "DICOM_SCP")

  -callingae string
    	AE title presented by this client (default "DICOM_SCU")

  -cecho
    	Send a C-ECHO request to the destination

  -cfind
    	Send a C-FIND request to the destination

  -cmove
    	Send a C-MOVE request to the destination

  -cstore
    	Send a C-STORE request to the destination (requires -file)

  -datastore string
    	Root directory for SCP storage (required with -scp)

  -destinationae string
    	Move-destination AE title (required with -cmove)

  -dump
    	Dump all tags of a DICOM file to stdout (requires -file)

  -file string
    	Path to a DICOM file (used with -cstore and -dump)

  -host string
    	Destination hostname or IP address (default "localhost")

  -port int
    	Destination TCP port (default 1040)

  -query string
    	Comma-separated tag=value pairs added to the request, e.g. 00080020=20260101

  -scp
    	Start an SCP server; blocks until SIGINT/SIGTERM

  -studyuid string
    	Study Instance UID added to the request (used with -cfind, -cmove)

  -tls
    	Enable TLS. For -scp: requires -tlscert and -tlskey. For SCU operations:
    	uses the system certificate pool unless -tlsca is provided.

  -tlscert string
    	Path to a PEM-encoded TLS certificate file (required when -scp -tls is set;
    	also used as the client certificate for mutual TLS SCU connections)

  -tlskey string
    	Path to a PEM-encoded TLS private key file (required when -scp -tls is set)

  -tlsca string
    	Path to a PEM-encoded CA certificate file used to verify the remote peer.
    	For -scp: enables mutual TLS (RequireAndVerifyClientCert).
    	For SCU: overrides the system certificate pool.

  -tlsinsecure
    	Skip TLS certificate verification for outbound SCU connections.
    	Do not use in production.
