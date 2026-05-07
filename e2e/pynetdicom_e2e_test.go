//go:build e2e

package e2e

// Tests for io-dicom SCU functionality against a pynetdicom SCP.
//
// pynetdicom is a pure-Python DICOM implementation (https://pydicom.github.io/pynetdicom/)
// that provides a completely independent protocol stack from DCMTK, making these
// tests valuable for catching interoperability issues that DCMTK tests might miss.
//
// Prerequisites:
//
//	pip3 install pynetdicom sqlalchemy
//
// Run with:
//
//	go test -tags e2e -v ./e2e/... -run Pynetdicom

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomstatus"
	"github.com/innovative-io/io-dicom/services"
)

// TestIoDicomEchoAgainstPynetdicom verifies that io-dicom's EchoSCU successfully
// sends a C-ECHO to a running pynetdicom storescp (which handles Verification
// SOP Class by default).
func TestIoDicomEchoAgainstPynetdicom(t *testing.T) {
	checkPynetdicomAvailable(t)

	port := mustFreePort(t)
	tmpDir := t.TempDir()

	var scpLogs bytes.Buffer
	stop := startPynetdicomStoreSCP(t, "PYNET_SCP", port, tmpDir, &scpLogs)
	defer stop()

	waitForEchoWithIoDicom(t, port, "PYNET_SCP", 15*time.Second)

	dest := &network.Destination{
		Name:      "pynet-echo",
		CalledAE:  "PYNET_SCP",
		CallingAE: "IO_DICOM_SCU",
		HostName:  "127.0.0.1",
		Port:      port,
	}
	scu := services.NewSCU(dest)
	if err := scu.EchoSCU(context.Background(), 10); err != nil {
		t.Fatalf("EchoSCU against pynetdicom storescp: %v\nstorescp logs:\n%s", err, scpLogs.String())
	}
}

// TestIoDicomStoreAgainstPynetdicom verifies that io-dicom's StoreSCU can send a
// DICOM file to a pynetdicom storescp and that the file is received on disk.
func TestIoDicomStoreAgainstPynetdicom(t *testing.T) {
	checkPynetdicomAvailable(t)

	samplePath := sampleDICOMPath(t)
	port := mustFreePort(t)
	tmpDir := t.TempDir()

	var scpLogs bytes.Buffer
	stop := startPynetdicomStoreSCP(t, "PYNET_SCP", port, tmpDir, &scpLogs)
	defer stop()

	waitForEchoWithIoDicom(t, port, "PYNET_SCP", 15*time.Second)

	dest := &network.Destination{
		Name:      "pynet-store",
		CalledAE:  "PYNET_SCP",
		CallingAE: "IO_DICOM_SCU",
		HostName:  "127.0.0.1",
		Port:      port,
	}
	scu := services.NewSCU(dest)
	if err := scu.StoreSCU(context.Background(), samplePath, 30); err != nil {
		t.Fatalf("StoreSCU against pynetdicom storescp: %v\nstorescp logs:\n%s", err, scpLogs.String())
	}

	if err := waitForFileInDir(tmpDir, 10*time.Second); err != nil {
		t.Fatalf("pynetdicom storescp did not receive file: %v\nstorescp logs:\n%s", err, scpLogs.String())
	}
}

// TestIoDicomFindAgainstPynetdicom verifies that io-dicom's FindSCU returns
// matches from a pynetdicom qrscp that has been populated with a known study.
func TestIoDicomFindAgainstPynetdicom(t *testing.T) {
	checkPynetdicomQRSCPAvailable(t)

	samplePath := sampleDICOMPath(t)
	studyUID := extractStudyUID(t, samplePath)

	qrPort := mustFreePort(t)
	tmpDir := t.TempDir()
	instanceDir := filepath.Join(tmpDir, "instances")
	dbPath := filepath.Join(tmpDir, "instances.db")

	cfgPath := writePynetdicomQRSCPConfig(t, tmpDir, instanceDir, dbPath, qrPort, 0)
	var qrLogs bytes.Buffer
	stopQR := startPynetdicomQRSCP(t, cfgPath, &qrLogs)
	defer stopQR()

	waitForEchoWithIoDicom(t, qrPort, "PYNET_QR", 20*time.Second)

	// Populate qrscp via io-dicom StoreSCU — qrscp acts as a Storage SCP
	// and indexes received instances in its SQLite database.
	storeDest := &network.Destination{
		Name:      "pynet-qr-populate",
		CalledAE:  "PYNET_QR",
		CallingAE: "IO_DICOM_SCU",
		HostName:  "127.0.0.1",
		Port:      qrPort,
	}
	if err := services.NewSCU(storeDest).StoreSCU(context.Background(), samplePath, 30); err != nil {
		t.Fatalf("StoreSCU to pynetdicom qrscp: %v\nqrscp logs:\n%s", err, qrLogs.String())
	}

	dest := &network.Destination{
		Name:      "pynet-qr-find",
		CalledAE:  "PYNET_QR",
		CallingAE: "IO_DICOM_SCU",
		HostName:  "127.0.0.1",
		Port:      qrPort,
	}
	scu := services.NewSCU(dest)

	findCount := 0
	scu.SetOnCFindResult(func(result media.DICOMObject) {
		if result != nil {
			findCount++
		}
	})

	findQuery := media.NewEmptyDCMObj()
	findQuery.Write(tags.QueryRetrieveLevel, "STUDY")
	findQuery.Write(tags.StudyInstanceUID, studyUID)

	_, findStatus, err := scu.FindSCU(context.Background(), findQuery, 15)
	if err != nil {
		t.Fatalf("FindSCU against pynetdicom qrscp: %v\nqrscp logs:\n%s", err, qrLogs.String())
	}
	if findStatus != dicomstatus.Success {
		t.Fatalf("FindSCU status=0x%04X want=0x%04X\nqrscp logs:\n%s", findStatus, dicomstatus.Success, qrLogs.String())
	}
	if findCount == 0 {
		t.Fatalf("FindSCU returned no matches for StudyInstanceUID=%s\nqrscp logs:\n%s", studyUID, qrLogs.String())
	}
}

// TestIoDicomMoveAgainstPynetdicom verifies the full C-FIND → C-MOVE flow using
// io-dicom's SCU against pynetdicom's qrscp, with a pynetdicom storescp as the
// C-MOVE sub-operation destination.
//
// Flow:
//  1. Start pynetdicom storescp as the C-MOVE destination (MOVE_DEST).
//  2. Start pynetdicom qrscp (PYNET_QR) with MOVE_DEST configured, and populate
//     it via io-dicom StoreSCU.
//  3. io-dicom FindSCU queries qrscp — expects the stored study.
//  4. io-dicom MoveSCU requests qrscp to send to MOVE_DEST.
//  5. Verify that MOVE_DEST storescp received the instance.
func TestIoDicomMoveAgainstPynetdicom(t *testing.T) {
	checkPynetdicomQRSCPAvailable(t)

	samplePath := sampleDICOMPath(t)
	studyUID := extractStudyUID(t, samplePath)

	qrPort := mustFreePort(t)
	moveDestPort := mustFreePort(t)
	tmpDir := t.TempDir()
	instanceDir := filepath.Join(tmpDir, "instances")
	dbPath := filepath.Join(tmpDir, "instances.db")
	moveDestDir := filepath.Join(tmpDir, "move_dest")
	if err := os.MkdirAll(moveDestDir, 0o755); err != nil {
		t.Fatalf("mkdir moveDestDir: %v", err)
	}

	// Start pynetdicom storescp as the move destination.
	var moveDestLogs bytes.Buffer
	stopMoveDest := startPynetdicomStoreSCP(t, "MOVE_DEST", moveDestPort, moveDestDir, &moveDestLogs)
	defer stopMoveDest()

	// Start pynetdicom qrscp with MOVE_DEST configured.
	cfgPath := writePynetdicomQRSCPConfig(t, tmpDir, instanceDir, dbPath, qrPort, moveDestPort)
	var qrLogs bytes.Buffer
	stopQR := startPynetdicomQRSCP(t, cfgPath, &qrLogs)
	defer stopQR()

	waitForEchoWithIoDicom(t, qrPort, "PYNET_QR", 20*time.Second)

	// Populate qrscp via io-dicom StoreSCU.
	storeDest := &network.Destination{
		Name:      "pynet-qr-populate",
		CalledAE:  "PYNET_QR",
		CallingAE: "IO_DICOM_SCU",
		HostName:  "127.0.0.1",
		Port:      qrPort,
	}
	if err := services.NewSCU(storeDest).StoreSCU(context.Background(), samplePath, 30); err != nil {
		t.Fatalf("StoreSCU to pynetdicom qrscp: %v\nqrscp logs:\n%s", err, qrLogs.String())
	}

	dest := &network.Destination{
		Name:      "pynet-qr-move",
		CalledAE:  "PYNET_QR",
		CallingAE: "IO_DICOM_SCU",
		HostName:  "127.0.0.1",
		Port:      qrPort,
	}
	scu := services.NewSCU(dest)

	// C-FIND to confirm data is indexed.
	findCount := 0
	scu.SetOnCFindResult(func(result media.DICOMObject) {
		if result != nil {
			findCount++
		}
	})
	findQuery := media.NewEmptyDCMObj()
	findQuery.Write(tags.QueryRetrieveLevel, "STUDY")
	findQuery.Write(tags.StudyInstanceUID, studyUID)

	_, findStatus, err := scu.FindSCU(context.Background(), findQuery, 15)
	if err != nil {
		t.Fatalf("FindSCU against pynetdicom qrscp: %v\nqrscp logs:\n%s", err, qrLogs.String())
	}
	if findStatus != dicomstatus.Success {
		t.Fatalf("FindSCU status=0x%04X want=0x%04X\nqrscp logs:\n%s", findStatus, dicomstatus.Success, qrLogs.String())
	}
	if findCount == 0 {
		t.Fatalf("FindSCU returned no matches\nqrscp logs:\n%s", qrLogs.String())
	}

	// C-MOVE: request qrscp to push the study to MOVE_DEST.
	moveQuery := media.NewEmptyDCMObj()
	moveQuery.Write(tags.QueryRetrieveLevel, "STUDY")
	moveQuery.Write(tags.StudyInstanceUID, studyUID)

	moveStatus, err := scu.MoveSCU(context.Background(), "MOVE_DEST", moveQuery, 30)
	if err != nil {
		t.Fatalf("MoveSCU against pynetdicom qrscp: %v\nqrscp logs:\n%s\nmovedest logs:\n%s",
			err, qrLogs.String(), moveDestLogs.String())
	}
	if moveStatus != dicomstatus.Success {
		t.Fatalf("MoveSCU status=0x%04X want=0x%04X\nqrscp logs:\n%s", moveStatus, dicomstatus.Success, qrLogs.String())
	}

	if err := waitForFileInDir(moveDestDir, 15*time.Second); err != nil {
		t.Fatalf("MOVE_DEST storescp did not receive any instances: %v\nqrscp logs:\n%s\nmovedest logs:\n%s",
			err, qrLogs.String(), moveDestLogs.String())
	}
}

// ─── pynetdicom helpers ──────────────────────────────────────────────────────

// checkPynetdicomAvailable skips if python3 or pynetdicom is not available.
func checkPynetdicomAvailable(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not found — skipping pynetdicom e2e tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "python3", "-c", "import pynetdicom").CombinedOutput(); err != nil {
		t.Skipf("pynetdicom not available — install with: pip3 install pynetdicom sqlalchemy\n%s", string(out))
	}
}

// checkPynetdicomQRSCPAvailable skips if pynetdicom or sqlalchemy is not available
// (sqlalchemy is required by pynetdicom qrscp).
func checkPynetdicomQRSCPAvailable(t *testing.T) {
	t.Helper()
	checkPynetdicomAvailable(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "python3", "-c", "import sqlalchemy").CombinedOutput(); err != nil {
		t.Skipf("sqlalchemy not available (required by pynetdicom qrscp) — install with: pip3 install sqlalchemy\n%s", string(out))
	}
}

// startPynetdicomStoreSCP starts a pynetdicom storescp and returns a cleanup func.
// It handles both Verification (C-ECHO) and Storage (C-STORE) SOP classes.
// Received DICOM files are written to outDir.
func startPynetdicomStoreSCP(t *testing.T, aeTitle string, port int, outDir string, logs *bytes.Buffer) func() {
	t.Helper()
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir storescp outDir: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "python3",
		"-m", "pynetdicom", "storescp",
		strconv.Itoa(port),
		"-aet", aeTitle,
		"-od", outDir,
	)
	cmd.Stdout = logs
	cmd.Stderr = logs
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start pynetdicom storescp (AE=%s port=%d): %v", aeTitle, port, err)
	}
	doneCh := make(chan error, 1)
	go func() { doneCh <- cmd.Wait() }()
	return func() {
		cancel()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		select {
		case <-doneCh:
		case <-time.After(3 * time.Second):
		}
	}
}

// writePynetdicomQRSCPConfig writes a qrscp .ini config file and returns its path.
// Pass moveDestPort=0 to omit the move-destination section (for Find-only tests).
func writePynetdicomQRSCPConfig(t *testing.T, tmpDir, instanceDir, dbPath string, port, moveDestPort int) string {
	t.Helper()
	if err := os.MkdirAll(instanceDir, 0o755); err != nil {
		t.Fatalf("mkdir qrscp instanceDir: %v", err)
	}

	cfg := fmt.Sprintf(`[DEFAULT]
ae_title: PYNET_QR
port: %d
max_pdu: 16382
acse_timeout: 30
dimse_timeout: 30
network_timeout: 30
bind_address:
instance_location: %s
database_location: %s
log_identifier: False
`, port, instanceDir, dbPath)

	if moveDestPort > 0 {
		cfg += fmt.Sprintf(`
[MOVE_DEST]
address: 127.0.0.1
port: %d
`, moveDestPort)
	}

	cfgPath := filepath.Join(tmpDir, "qrscp.ini")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write qrscp.ini: %v", err)
	}
	return cfgPath
}

// startPynetdicomQRSCP starts a pynetdicom qrscp process using the given config file.
// The config must specify ae_title and port (written by writePynetdicomQRSCPConfig).
func startPynetdicomQRSCP(t *testing.T, cfgPath string, logs *bytes.Buffer) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "python3",
		"-m", "pynetdicom", "qrscp",
		"-c", cfgPath,
	)
	cmd.Stdout = logs
	cmd.Stderr = logs
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start pynetdicom qrscp: %v", err)
	}
	doneCh := make(chan error, 1)
	go func() { doneCh <- cmd.Wait() }()
	return func() {
		cancel()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		select {
		case <-doneCh:
		case <-time.After(3 * time.Second):
		}
	}
}

// waitForEchoWithIoDicom polls using io-dicom's own EchoSCU until the SCP responds
// or the timeout expires. This works for any DICOM SCP that handles Verification.
func waitForEchoWithIoDicom(t *testing.T, port int, calledAE string, timeout time.Duration) {
	t.Helper()
	dest := &network.Destination{
		Name:      "readiness-echo",
		CalledAE:  calledAE,
		CallingAE: "IO_DICOM_SCU",
		HostName:  "127.0.0.1",
		Port:      port,
	}
	deadline := time.Now().Add(timeout)
	for {
		if err := services.NewSCU(dest).EchoSCU(context.Background(), 3); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %s on port %d to respond to C-ECHO", calledAE, port)
		}
		time.Sleep(200 * time.Millisecond)
	}
}
