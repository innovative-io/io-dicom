//go:build e2e

package test

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomstatus"
	"github.com/innovative-io/io-dicom/services"
)

// TestIoDicomEchoAgainstDCMTK verifies that io-dicom's EchoSCU successfully
// sends a C-ECHO to a running DCMTK storescp.
func TestIoDicomEchoAgainstDCMTK(t *testing.T) {
	checkDCMTKAvailable(t)

	port := mustFreePort(t)
	tmpDir := t.TempDir()

	var scpLogs bytes.Buffer
	stop := startDCMTKStoreSCP(t, "DCMTK_SCP", port, tmpDir, &scpLogs)
	defer stop()

	waitForEchoDCMTK(t, port, "DCMTK_SCP", 15*time.Second)

	dest := &network.Destination{
		Name:      "dcmtk-echo",
		CalledAE:  "DCMTK_SCP",
		CallingAE: "IO_DICOM_SCU",
		HostName:  "127.0.0.1",
		Port:      port,
	}
	scu := services.NewSCU(dest)
	if err := scu.EchoSCU(context.Background(), 10); err != nil {
		t.Fatalf("EchoSCU against DCMTK storescp: %v\nstorescp logs:\n%s", err, scpLogs.String())
	}
}

// TestIoDicomStoreAgainstDCMTK verifies that io-dicom's StoreSCU can send a
// DICOM file to a DCMTK storescp and that the file is received on disk.
func TestIoDicomStoreAgainstDCMTK(t *testing.T) {
	checkDCMTKAvailable(t)

	samplePath := sampleDICOMPath(t)
	port := mustFreePort(t)
	tmpDir := t.TempDir()

	var scpLogs bytes.Buffer
	stop := startDCMTKStoreSCP(t, "DCMTK_SCP", port, tmpDir, &scpLogs)
	defer stop()

	waitForEchoDCMTK(t, port, "DCMTK_SCP", 15*time.Second)

	dest := &network.Destination{
		Name:      "dcmtk-store",
		CalledAE:  "DCMTK_SCP",
		CallingAE: "IO_DICOM_SCU",
		HostName:  "127.0.0.1",
		Port:      port,
	}
	scu := services.NewSCU(dest)
	if err := scu.StoreSCU(context.Background(), samplePath, 30); err != nil {
		t.Fatalf("StoreSCU against DCMTK storescp: %v\nstorescp logs:\n%s", err, scpLogs.String())
	}

	if err := waitForFileInDir(tmpDir, 10*time.Second); err != nil {
		t.Fatalf("DCMTK storescp did not receive the file: %v\nstorescp logs:\n%s", err, scpLogs.String())
	}
}

// TestIoDicomFindAgainstDCMTK verifies that io-dicom's FindSCU returns matches
// from a DCMTK dcmqrscp that has been populated with a known study.
func TestIoDicomFindAgainstDCMTK(t *testing.T) {
	checkDCMTKAvailable(t)
	checkDCMTKQRSCPAvailable(t)

	samplePath := sampleDICOMPath(t)
	studyUID := extractStudyUID(t, samplePath)

	qrPort := mustFreePort(t)
	tmpDir := t.TempDir()
	storeDir := filepath.Join(tmpDir, "qr_store")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatalf("mkdir storeDir: %v", err)
	}

	cfgPath := writeDcmQrscpConfig(t, tmpDir, storeDir, qrPort, 0)
	var qrLogs bytes.Buffer
	stopQR := startDCMTKQRSCP(t, cfgPath, qrPort, &qrLogs)
	defer stopQR()

	waitForEchoDCMTK(t, qrPort, "DCMTK_QR", 20*time.Second)

	// Populate dcmqrscp via DCMTK storescu.
	runDCMTKCmd(t, 20*time.Second, "storescu",
		"-aet", "IO_DICOM_SCU",
		"-aec", "DCMTK_QR",
		"127.0.0.1", strconv.Itoa(qrPort),
		samplePath,
	)

	dest := &network.Destination{
		Name:      "dcmtk-qr-find",
		CalledAE:  "DCMTK_QR",
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
		t.Fatalf("FindSCU against DCMTK dcmqrscp: %v\nqrscp logs:\n%s", err, qrLogs.String())
	}
	if findStatus != dicomstatus.Success {
		t.Fatalf("FindSCU status=0x%04X want=0x%04X\nqrscp logs:\n%s", findStatus, dicomstatus.Success, qrLogs.String())
	}
	if findCount == 0 {
		t.Fatalf("FindSCU returned no matches for StudyInstanceUID=%s\nqrscp logs:\n%s", studyUID, qrLogs.String())
	}
}

// TestIoDicomMoveAgainstDCMTK verifies the full C-FIND → C-MOVE flow using
// io-dicom's SCU against DCMTK's dcmqrscp.
//
// Flow:
//  1. Start DCMTK storescp as the C-MOVE destination (MOVE_DEST).
//  2. Start dcmqrscp (DCMTK_QR) and populate it via DCMTK storescu.
//  3. io-dicom FindSCU queries dcmqrscp — expects the stored study.
//  4. io-dicom MoveSCU requests dcmqrscp to send to MOVE_DEST.
//  5. Verify that MOVE_DEST received the instance.
func TestIoDicomMoveAgainstDCMTK(t *testing.T) {
	checkDCMTKAvailable(t)
	checkDCMTKQRSCPAvailable(t)

	samplePath := sampleDICOMPath(t)
	studyUID := extractStudyUID(t, samplePath)

	qrPort := mustFreePort(t)
	moveDestPort := mustFreePort(t)
	tmpDir := t.TempDir()
	storeDir := filepath.Join(tmpDir, "qr_store")
	moveDestDir := filepath.Join(tmpDir, "move_dest")
	for _, d := range []string{storeDir, moveDestDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	var moveDestLogs bytes.Buffer
	stopMoveDest := startDCMTKStoreSCP(t, "MOVE_DEST", moveDestPort, moveDestDir, &moveDestLogs)
	defer stopMoveDest()

	cfgPath := writeDcmQrscpConfig(t, tmpDir, storeDir, qrPort, moveDestPort)
	var qrLogs bytes.Buffer
	stopQR := startDCMTKQRSCP(t, cfgPath, qrPort, &qrLogs)
	defer stopQR()

	waitForEchoDCMTK(t, qrPort, "DCMTK_QR", 20*time.Second)

	// Populate dcmqrscp via DCMTK storescu.
	runDCMTKCmd(t, 20*time.Second, "storescu",
		"-aet", "IO_DICOM_SCU",
		"-aec", "DCMTK_QR",
		"127.0.0.1", strconv.Itoa(qrPort),
		samplePath,
	)

	dest := &network.Destination{
		Name:      "dcmtk-qr-move",
		CalledAE:  "DCMTK_QR",
		CallingAE: "IO_DICOM_SCU",
		HostName:  "127.0.0.1",
		Port:      qrPort,
	}
	scu := services.NewSCU(dest)

	// C-FIND first to confirm data is present.
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
		t.Fatalf("FindSCU against DCMTK dcmqrscp: %v\nqrscp logs:\n%s", err, qrLogs.String())
	}
	if findStatus != dicomstatus.Success {
		t.Fatalf("FindSCU status=0x%04X want=0x%04X\nqrscp logs:\n%s", findStatus, dicomstatus.Success, qrLogs.String())
	}
	if findCount == 0 {
		t.Fatalf("FindSCU returned no matches for StudyInstanceUID=%s\nqrscp logs:\n%s", studyUID, qrLogs.String())
	}

	// C-MOVE.
	moveQuery := media.NewEmptyDCMObj()
	moveQuery.Write(tags.QueryRetrieveLevel, "STUDY")
	moveQuery.Write(tags.StudyInstanceUID, studyUID)

	moveStatus, err := scu.MoveSCU(context.Background(), "MOVE_DEST", moveQuery, 30)
	if err != nil {
		t.Fatalf("MoveSCU against DCMTK dcmqrscp: %v\nqrscp logs:\n%s", err, qrLogs.String())
	}
	if moveStatus != dicomstatus.Success {
		t.Fatalf("MoveSCU status=0x%04X want=0x%04X\nqrscp logs:\n%s", moveStatus, dicomstatus.Success, qrLogs.String())
	}

	if err := waitForFileInDir(moveDestDir, 15*time.Second); err != nil {
		t.Fatalf("MOVE_DEST storescp did not receive any instances: %v\nqrscp logs:\n%s\nmovedest logs:\n%s",
			err, qrLogs.String(), moveDestLogs.String())
	}
}

// ─── helpers ────────────────────────────────────────────────────────────────

// checkDCMTKAvailable skips the test if core DCMTK CLI tools are not in PATH.
func checkDCMTKAvailable(t *testing.T) {
	t.Helper()
	for _, tool := range []string{"echoscu", "storescp", "storescu"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("DCMTK tool %q not found in PATH — skipping DCMTK e2e tests (install with: brew install dcmtk)", tool)
		}
	}
}

// checkDCMTKQRSCPAvailable skips the test if dcmqrscp is not available.
func checkDCMTKQRSCPAvailable(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("dcmqrscp"); err != nil {
		t.Skip("dcmqrscp not found in PATH — skipping Q/R tests (install with: brew install dcmtk)")
	}
}

// sampleDICOMPath returns the path to the sample DICOM file used for e2e tests.
// Set IO_DICOM_SAMPLE_FILE to override the default location.
func sampleDICOMPath(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("IO_DICOM_SAMPLE_FILE"); p != "" {
		return p
	}
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	candidate := filepath.Join(filepath.Dir(thisFile), "..", "testdata", "test.dcm")
	if _, err := os.Stat(candidate); err != nil {
		t.Fatalf("sample DICOM not found at %s (set IO_DICOM_SAMPLE_FILE to override): %v", candidate, err)
	}
	return candidate
}

// extractStudyUID uses dcmdump to extract (0020,000D) without importing
// the full io-dicom media library, keeping the helper self-contained.
func extractStudyUID(t *testing.T, path string) string {
	t.Helper()
	obj, err := media.NewDCMObjFromFile(path)
	if err != nil {
		t.Fatalf("read sample DICOM: %v", err)
	}
	uid := obj.GetString(tags.StudyInstanceUID)
	if uid == "" {
		t.Fatal("sample DICOM missing StudyInstanceUID")
	}
	return uid
}

// mustFreePort allocates and immediately releases a free TCP port.
func mustFreePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// startDCMTKStoreSCP starts a DCMTK storescp process and returns a stop func.
// +xa makes it accept all presentation contexts; it saves received files to outDir.
func startDCMTKStoreSCP(t *testing.T, aeTitle string, port int, outDir string, logs *bytes.Buffer) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "storescp",
		"+xa",
		"-aet", aeTitle,
		"-od", outDir,
		strconv.Itoa(port),
	)
	cmd.Stdout = logs
	cmd.Stderr = logs
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start storescp (AE=%s port=%d): %v", aeTitle, port, err)
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

// writeDcmQrscpConfig generates a minimal dcmqrscp.cfg in tmpDir and returns its path.
// Pass moveDestPort=0 to omit the move-destination HostTable entry (for Find-only tests).
func writeDcmQrscpConfig(t *testing.T, tmpDir, storeDir string, qrPort, moveDestPort int) string {
	t.Helper()

	hostTable := ""
	if moveDestPort > 0 {
		hostTable = fmt.Sprintf("move_dest = (MOVE_DEST, localhost, %d)\n", moveDestPort)
	}

	cfg := fmt.Sprintf(`NetworkTCPPort  = %d
MaxPDUSize      = 16384
MaxAssociations = 16

HostTable BEGIN
%sHostTable END

VendorTable BEGIN
VendorTable END

AETable BEGIN
DCMTK_QR  %s  RW  (200, 1024mb)  ANY
AETable END
`, qrPort, hostTable, storeDir)

	cfgPath := filepath.Join(tmpDir, "dcmqrscp.cfg")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write dcmqrscp.cfg: %v", err)
	}
	return cfgPath
}

// startDCMTKQRSCP starts a dcmqrscp process using the given config file.
func startDCMTKQRSCP(t *testing.T, cfgPath string, port int, logs *bytes.Buffer) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "dcmqrscp",
		"-c", cfgPath,
		strconv.Itoa(port),
	)
	cmd.Stdout = logs
	cmd.Stderr = logs
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start dcmqrscp (port=%d): %v", port, err)
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

// waitForEchoDCMTK polls echoscu until the target AE responds or the timeout expires.
func waitForEchoDCMTK(t *testing.T, port int, calledAE string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		cmd := exec.CommandContext(ctx, "echoscu",
			"-aet", "IO_DICOM_SCU",
			"-aec", calledAE,
			"127.0.0.1", strconv.Itoa(port),
		)
		err := cmd.Run()
		cancel()
		if err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %s on port %d to respond to echoscu", calledAE, port)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// runDCMTKCmd runs a DCMTK CLI command and fails the test if it exits non-zero.
func runDCMTKCmd(t *testing.T, timeout time.Duration, name string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s failed: %v\nargs: %v\noutput:\n%s", name, err, args, string(out))
	}
	return string(out)
}

// waitForFileInDir polls dir until at least one non-directory file appears.
func waitForFileInDir(dir string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if !e.IsDir() {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("no files appeared in %s within %s", dir, timeout)
		}
		time.Sleep(200 * time.Millisecond)
	}
}
