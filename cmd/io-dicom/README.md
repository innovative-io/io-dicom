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
