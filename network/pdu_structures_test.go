package network

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/innovative-io/io-dicom/media"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// captureWrite flushes a Write call to a byte buffer and returns the bytes.
func captureWrite(fn func(rw *bufio.ReadWriter) error) ([]byte, error) {
	var buf bytes.Buffer
	rw := bufio.NewReadWriter(bufio.NewReader(&buf), bufio.NewWriter(&buf))
	if err := fn(rw); err != nil {
		return nil, err
	}
	rw.Flush()
	return buf.Bytes(), nil
}

// captureWriteBool is like captureWrite but for functions returning bool.
func captureWriteBool(fn func(rw *bufio.ReadWriter) bool) ([]byte, bool) {
	var buf bytes.Buffer
	rw := bufio.NewReadWriter(bufio.NewReader(&buf), bufio.NewWriter(&buf))
	ok := fn(rw)
	rw.Flush()
	return buf.Bytes(), ok
}

// ── AbortRequest ─────────────────────────────────────────────────────────────

func TestAbortRequest_Size(t *testing.T) {
	a := NewAbortRequest()
	if got := a.Size(); got != 10 {
		t.Errorf("AbortRequest.Size() = %d, want 10", got)
	}
}

func TestAbortRequest_GetReason_Default(t *testing.T) {
	a := NewAbortRequest()
	if r := a.GetReason(); r == "" {
		t.Error("AbortRequest.GetReason() should not be empty for reason 0")
	}
}

func TestAbortRequest_GetReason_ProviderReason(t *testing.T) {
	a := NewAbortRequest().(*abortRequest)
	a.Source = 2
	a.Reason = 2
	if r := a.GetReason(); r != "Unexpected PDU" {
		t.Errorf("AbortRequest.GetReason() = %q, want 'Unexpected PDU'", r)
	}
}

func TestAbortRequest_GetReason_UnknownReasonFallback(t *testing.T) {
	a := NewAbortRequest().(*abortRequest)
	a.Source = 2
	a.Reason = 99
	if r := a.GetReason(); r != "No reason given" {
		t.Errorf("AbortRequest.GetReason() = %q, want 'No reason given'", r)
	}
}

func TestAbortRequest_WriteRead_Roundtrip(t *testing.T) {
	a := NewAbortRequest()
	data, err := captureWrite(a.Write)
	if err != nil {
		t.Fatalf("AbortRequest.Write: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("AbortRequest.Write produced no bytes")
	}

	buf := media.NewDICOMBufferFromBytes(data)
	a2 := NewAbortRequest()
	if err := a2.Read(buf); err != nil {
		t.Fatalf("AbortRequest.Read: %v", err)
	}
}

func TestAbortRequest_ReadDynamic_Roundtrip(t *testing.T) {
	a := NewAbortRequest()
	data, err := captureWrite(a.Write)
	if err != nil {
		t.Fatalf("AbortRequest.Write: %v", err)
	}
	// ReadDynamic skips the first ItemType byte
	buf := media.NewDICOMBufferFromBytes(data[1:])
	a2 := NewAbortRequest()
	if err := a2.ReadDynamic(buf); err != nil {
		t.Fatalf("AbortRequest.ReadDynamic: %v", err)
	}
}

// ── AssociationReject ─────────────────────────────────────────────────────────

func TestAssociationReject_Set_GetReason(t *testing.T) {
	rj := NewAssociationReject()
	rj.Set(1, 1, 7) // UL-service-user, Called AE not recognised
	r := rj.GetReason()
	if r != "Called AE not recognized" {
		t.Errorf("AssociationReject.GetReason() = %q, want 'Called AE not recognized'", r)
	}
}

func TestAssociationReject_Set_TransientReason(t *testing.T) {
	rj := NewAssociationReject()
	rj.Set(2, 3, 1) // transient, Source 3 presentation, Temporary congestion
	r := rj.GetReason()
	if r != "Temporary congestion" {
		t.Errorf("AssociationReject.GetReason() = %q, want 'Temporary congestion'", r)
	}
}

func TestAssociationReject_Set_ACSEProtocolVersionNotSupportedReason(t *testing.T) {
	rj := NewAssociationReject()
	rj.Set(1, 2, 2) // Source 2 ACSE, Protocol version not supported
	r := rj.GetReason()
	if r != "Protocol version not supported" {
		t.Errorf("AssociationReject.GetReason() = %q, want 'Protocol version not supported'", r)
	}
}

func TestAssociationReject_Set_PresentationLocalLimitExceededReason(t *testing.T) {
	rj := NewAssociationReject()
	rj.Set(2, 3, 2) // Source 3 presentation, Local limit exceeded
	r := rj.GetReason()
	if r != "Local limit exceeded" {
		t.Errorf("AssociationReject.GetReason() = %q, want 'Local limit exceeded'", r)
	}
}

func TestAssociationReject_Set_UnknownReasonFallsBack(t *testing.T) {
	rj := NewAssociationReject()
	rj.Set(1, 1, 99)
	r := rj.GetReason()
	if r != "No reason given" {
		t.Errorf("AssociationReject.GetReason() = %q, want 'No reason given'", r)
	}
}

func TestAssociationReject_Size(t *testing.T) {
	rj := NewAssociationReject()
	if got := rj.Size(); got != 10 {
		t.Errorf("AssociationReject.Size() = %d, want 10", got)
	}
}

func TestAssociationReject_WriteRead_Roundtrip(t *testing.T) {
	rj := NewAssociationReject()
	rj.Set(1, 1, 3)
	data, err := captureWrite(rj.Write)
	if err != nil {
		t.Fatalf("AssociationReject.Write: %v", err)
	}

	buf := media.NewDICOMBufferFromBytes(data)
	rj2 := NewAssociationReject()
	if err := rj2.Read(buf); err != nil {
		t.Fatalf("AssociationReject.Read: %v", err)
	}
	if rj2.GetReason() != rj.GetReason() {
		t.Errorf("AssociationReject roundtrip: reason mismatch got %q want %q", rj2.GetReason(), rj.GetReason())
	}
}

func TestAssociationReject_ReadDynamic_Roundtrip(t *testing.T) {
	rj := NewAssociationReject()
	data, err := captureWrite(rj.Write)
	if err != nil {
		t.Fatalf("AssociationReject.Write: %v", err)
	}
	buf := media.NewDICOMBufferFromBytes(data[1:])
	rj2 := NewAssociationReject()
	if err := rj2.ReadDynamic(buf); err != nil {
		t.Fatalf("AssociationReject.ReadDynamic: %v", err)
	}
}

// ── ReleaseRequest ────────────────────────────────────────────────────────────

func TestReleaseRequest_Size(t *testing.T) {
	r := NewReleaseRequest()
	if got := r.Size(); got != 10 {
		t.Errorf("ReleaseRequest.Size() = %d, want 10", got)
	}
}

func TestReleaseRequest_WriteRead_Roundtrip(t *testing.T) {
	r := NewReleaseRequest()
	data, err := captureWrite(r.Write)
	if err != nil {
		t.Fatalf("ReleaseRequest.Write: %v", err)
	}

	buf := media.NewDICOMBufferFromBytes(data)
	r2 := NewReleaseRequest()
	if err := r2.Read(buf); err != nil {
		t.Fatalf("ReleaseRequest.Read: %v", err)
	}
}

func TestReleaseRequest_ReadDynamic_Roundtrip(t *testing.T) {
	r := NewReleaseRequest()
	data, err := captureWrite(r.Write)
	if err != nil {
		t.Fatalf("ReleaseRequest.Write: %v", err)
	}
	buf := media.NewDICOMBufferFromBytes(data[1:])
	r2 := NewReleaseRequest()
	if err := r2.ReadDynamic(buf); err != nil {
		t.Fatalf("ReleaseRequest.ReadDynamic: %v", err)
	}
}

// ── ReleaseResponse ───────────────────────────────────────────────────────────

func TestReleaseResponse_Size(t *testing.T) {
	r := NewReleaseResponse()
	if got := r.Size(); got != 10 {
		t.Errorf("ReleaseResponse.Size() = %d, want 10", got)
	}
}

func TestReleaseResponse_WriteRead_Roundtrip(t *testing.T) {
	r := NewReleaseResponse()
	data, err := captureWrite(r.Write)
	if err != nil {
		t.Fatalf("ReleaseResponse.Write: %v", err)
	}

	buf := media.NewDICOMBufferFromBytes(data)
	r2 := NewReleaseResponse()
	if err := r2.Read(buf); err != nil {
		t.Fatalf("ReleaseResponse.Read: %v", err)
	}
}

func TestReleaseResponse_ReadDynamic_Roundtrip(t *testing.T) {
	r := NewReleaseResponse()
	data, err := captureWrite(r.Write)
	if err != nil {
		t.Fatalf("ReleaseResponse.Write: %v", err)
	}
	buf := media.NewDICOMBufferFromBytes(data[1:])
	r2 := NewReleaseResponse()
	if err := r2.ReadDynamic(buf); err != nil {
		t.Fatalf("ReleaseResponse.ReadDynamic: %v", err)
	}
}

// ── MaximumPDULength ──────────────────────────────────────────────────────────

func TestMaximumPDULength_GetSetMaximumLength(t *testing.T) {
	m := newMaximumPDULength()
	m.SetMaximumLength(65536)
	if got := m.GetMaximumLength(); got != 65536 {
		t.Errorf("MaximumPDULength.GetMaximumLength() = %d, want 65536", got)
	}
}

func TestMaximumPDULength_Size(t *testing.T) {
	m := newMaximumPDULength()
	if got := m.Size(); got != 8 {
		t.Errorf("MaximumPDULength.Size() = %d, want 8", got)
	}
}

func TestMaximumPDULength_WriteRead_Roundtrip(t *testing.T) {
	m := newMaximumPDULength()
	m.SetMaximumLength(32768)

	var out bytes.Buffer
	rw := bufio.NewReadWriter(bufio.NewReader(&out), bufio.NewWriter(&out))
	if !m.Write(rw) {
		t.Fatal("MaximumPDULength.Write returned false")
	}
	rw.Flush()

	buf := media.NewDICOMBufferFromBytes(out.Bytes())
	m2 := newMaximumPDULength()
	if err := m2.Read(buf); err != nil {
		t.Fatalf("MaximumPDULength.Read: %v", err)
	}
	if m2.GetMaximumLength() != 32768 {
		t.Errorf("MaximumPDULength roundtrip: got %d want 32768", m2.GetMaximumLength())
	}
}

func TestMaximumPDULength_ReadDynamic_Roundtrip(t *testing.T) {
	m := newMaximumPDULength()
	m.SetMaximumLength(1024)

	var out bytes.Buffer
	rw := bufio.NewReadWriter(bufio.NewReader(&out), bufio.NewWriter(&out))
	m.Write(rw)
	rw.Flush()

	// ReadDynamic skips ItemType byte
	buf := media.NewDICOMBufferFromBytes(out.Bytes()[1:])
	m2 := newMaximumPDULength()
	if err := m2.ReadDynamic(buf); err != nil {
		t.Fatalf("MaximumPDULength.ReadDynamic: %v", err)
	}
	if m2.GetMaximumLength() != 1024 {
		t.Errorf("MaximumPDULength ReadDynamic roundtrip: got %d want 1024", m2.GetMaximumLength())
	}
}

// ── AsyncOperationWindow ──────────────────────────────────────────────────────

func TestAsyncOperationWindow_Getters_ZeroDefault(t *testing.T) {
	a := newAsyncOperationWindow()
	if a.GetMaxNumberOperationsInvoked() != 0 {
		t.Error("AsyncOperationWindow: default MaxNumberOperationsInvoked should be 0")
	}
	if a.GetMaxNumberOperationsPerformed() != 0 {
		t.Error("AsyncOperationWindow: default MaxNumberOperationsPerformed should be 0")
	}
}

func TestAsyncOperationWindow_Size(t *testing.T) {
	a := newAsyncOperationWindow()
	// Length=0 so Size()=4
	if got := a.Size(); got != 4 {
		t.Errorf("AsyncOperationWindow.Size() = %d, want 4", got)
	}
}

func TestAsyncOperationWindow_ReadDynamic(t *testing.T) {
	// Manually build the byte payload: reserved(1) + length(2) + invoked(2) + performed(2) = 7 bytes
	data := []byte{
		0x00,       // reserved1
		0x00, 0x04, // length = 4
		0x00, 0x05, // MaxNumberOperationsInvoked = 5
		0x00, 0x03, // MaxNumberOperationsPerformed = 3
	}
	buf := media.NewDICOMBufferFromBytes(data)
	a := newAsyncOperationWindow()
	if err := a.ReadDynamic(buf); err != nil {
		t.Fatalf("AsyncOperationWindow.ReadDynamic: %v", err)
	}
	if a.GetMaxNumberOperationsInvoked() != 5 {
		t.Errorf("AsyncOperationWindow: invoked = %d want 5", a.GetMaxNumberOperationsInvoked())
	}
	if a.GetMaxNumberOperationsPerformed() != 3 {
		t.Errorf("AsyncOperationWindow: performed = %d want 3", a.GetMaxNumberOperationsPerformed())
	}
}

func TestAsyncOperationWindow_Read(t *testing.T) {
	// Full byte payload including ItemType byte
	data := []byte{
		0x53,       // ItemType
		0x00,       // reserved1
		0x00, 0x04, // length = 4
		0x00, 0x02, // invoked = 2
		0x00, 0x01, // performed = 1
	}
	buf := media.NewDICOMBufferFromBytes(data)
	a := newAsyncOperationWindow()
	if err := a.Read(buf); err != nil {
		t.Fatalf("AsyncOperationWindow.Read: %v", err)
	}
	if a.GetMaxNumberOperationsInvoked() != 2 {
		t.Errorf("AsyncOperationWindow.Read: invoked = %d want 2", a.GetMaxNumberOperationsInvoked())
	}
}

// ── RoleSelect ────────────────────────────────────────────────────────────────

func TestRoleSelect_Size_Zero(t *testing.T) {
	r := newRoleSelect()
	// Length=0, so Size()=4
	if got := r.Size(); got != 4 {
		t.Errorf("RoleSelect.Size() = %d, want 4", got)
	}
}

func TestRoleSelect_WriteRead_Roundtrip(t *testing.T) {
	r := newRoleSelect()
	data, ok := captureWriteBool(r.Write)
	if !ok {
		t.Fatal("RoleSelect.Write returned false")
	}
	if len(data) == 0 {
		t.Fatal("RoleSelect.Write produced no bytes")
	}

	buf := media.NewDICOMBufferFromBytes(data)
	r2 := newRoleSelect()
	if err := r2.Read(buf); err != nil {
		t.Fatalf("RoleSelect.Read: %v", err)
	}
}

func TestRoleSelect_ReadDynamic(t *testing.T) {
	uid := "1.2.840.10008.5.1.4.1.1.2"
	uidLen := uint16(len(uid))
	// Build: reserved(1) + length(2) + uidLength(2) + uid + scuRole(1) + scpRole(1)
	var out bytes.Buffer
	out.WriteByte(0x00) // reserved
	out.WriteByte(byte((4 + uidLen + 2) >> 8))
	out.WriteByte(byte(4 + uidLen + 2)) // length
	out.WriteByte(byte(uidLen >> 8))
	out.WriteByte(byte(uidLen))
	out.WriteString(uid)
	out.WriteByte(0x01) // SCURole
	out.WriteByte(0x01) // SCPRole

	buf := media.NewDICOMBufferFromBytes(out.Bytes())
	r := newRoleSelect()
	if err := r.ReadDynamic(buf); err != nil {
		t.Fatalf("RoleSelect.ReadDynamic: %v", err)
	}
}

// ── UIDItem extended ─────────────────────────────────────────────────────────

func TestUIDItem_SettersGetters(t *testing.T) {
	u := NewUIDItem("1.2.3", 0x10)
	u.SetType(0x30)
	u.SetReserved(0x01)
	u.SetUID("9.8.7.6")
	u.SetLength(7)

	if u.GetType() != 0x30 {
		t.Errorf("UIDItem.GetType() = %02X, want 0x30", u.GetType())
	}
	if u.GetReserved() != 0x01 {
		t.Errorf("UIDItem.GetReserved() = %02X, want 0x01", u.GetReserved())
	}
	if u.GetUID() != "9.8.7.6" {
		t.Errorf("UIDItem.GetUID() = %q, want '9.8.7.6'", u.GetUID())
	}
	if u.GetLength() != 7 {
		t.Errorf("UIDItem.GetLength() = %d, want 7", u.GetLength())
	}
	if u.GetSize() != 11 { // length + 4
		t.Errorf("UIDItem.GetSize() = %d, want 11", u.GetSize())
	}
}

func TestUIDItem_WriteRead_Roundtrip(t *testing.T) {
	uid := "1.2.840.10008.5.1.4.1.1.2"
	u := NewUIDItem(uid, 0x10)
	data, err := captureWrite(u.Write)
	if err != nil {
		t.Fatalf("UIDItem.Write: %v", err)
	}

	buf := media.NewDICOMBufferFromBytes(data)
	u2 := NewUIDItem("", 0x00)
	if err := u2.Read(buf); err != nil {
		t.Fatalf("UIDItem.Read: %v", err)
	}
	if u2.GetUID() != uid {
		t.Errorf("UIDItem roundtrip: UID = %q, want %q", u2.GetUID(), uid)
	}
}

func TestUIDItem_ReadDynamic_Roundtrip(t *testing.T) {
	uid := "1.2.3.4.5"
	u := NewUIDItem(uid, 0x52)
	data, err := captureWrite(u.Write)
	if err != nil {
		t.Fatalf("UIDItem.Write: %v", err)
	}
	// ReadDynamic skips ItemType byte
	buf := media.NewDICOMBufferFromBytes(data[1:])
	u2 := NewUIDItem("", 0x00)
	if err := u2.ReadDynamic(buf); err != nil {
		t.Fatalf("UIDItem.ReadDynamic: %v", err)
	}
	if u2.GetUID() != uid {
		t.Errorf("UIDItem ReadDynamic roundtrip: got %q want %q", u2.GetUID(), uid)
	}
}

// ── UserInformation extended ──────────────────────────────────────────────────

func TestUserInformation_SetImplementationClassUID(t *testing.T) {
	ui := newUserInformation()
	ui.SetImplementationClassUID("1.2.840.10008.5.1.4.34.5")
	if uid := ui.GetImplementationClass().GetUID(); uid != "1.2.840.10008.5.1.4.34.5" {
		t.Errorf("UserInformation.GetImplementationClass.GetUID() = %q", uid)
	}
}

func TestUserInformation_SetImplementationVersionName(t *testing.T) {
	ui := newUserInformation()
	ui.SetImplementationVersionName("IO-DICOM-2.0.0")
	if ver := ui.GetImplementationVersion().GetUID(); ver != "IO-DICOM-2.0.0" {
		t.Errorf("UserInformation.GetImplementationVersion.GetUID() = %q", ver)
	}
}

func TestUserInformation_GetSetItemType(t *testing.T) {
	ui := newUserInformation()
	ui.SetItemType(0x51)
	if got := ui.GetItemType(); got != 0x51 {
		t.Errorf("UserInformation.GetItemType() = 0x%X, want 0x51", got)
	}
}

func TestUserInformation_SetMaxSubLength(t *testing.T) {
	ui := newUserInformation()
	m := newMaximumPDULength()
	m.SetMaximumLength(16384)
	ui.SetMaxSubLength(m)
	if ui.GetMaxSubLength().GetMaximumLength() != 16384 {
		t.Errorf("UserInformation MaxSubLength roundtrip failed")
	}
}

func TestUserInformation_GetAsyncOperationWindow(t *testing.T) {
	ui := newUserInformation()
	if ui.GetAsyncOperationWindow() == nil {
		t.Error("UserInformation.GetAsyncOperationWindow() should not be nil")
	}
}

func TestUserInformation_Size(t *testing.T) {
	ui := newUserInformation()
	ui.SetImplementationClassUID("1.2.3")
	ui.SetImplementationVersionName("v1")
	sz := ui.Size()
	if sz == 0 {
		t.Error("UserInformation.Size() should not be zero")
	}
}
