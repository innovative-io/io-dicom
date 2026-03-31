package utils

import (
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/media"
)

func init() {
	media.InitDict()
}

func TestDefaultCFindRequest(t *testing.T) {
	obj := DefaultCFindRequest()
	if obj == nil {
		t.Fatal("DefaultCFindRequest() returned nil")
	}
	if obj.TagCount() == 0 {
		t.Fatal("DefaultCFindRequest() returned empty object")
	}
	level := obj.GetString(tags.QueryRetrieveLevel)
	if level != "STUDY" {
		t.Fatalf("QueryRetrieveLevel = %q, want STUDY", level)
	}
}

func TestDefaultCMoveRequest(t *testing.T) {
	obj := DefaultCMoveRequest("1.2.3.4.5")
	if obj == nil {
		t.Fatal("DefaultCMoveRequest() returned nil")
	}
	uid := obj.GetString(tags.StudyInstanceUID)
	if uid != "1.2.3.4.5" {
		t.Fatalf("StudyInstanceUID = %q, want 1.2.3.4.5", uid)
	}
	level := obj.GetString(tags.QueryRetrieveLevel)
	if level != "STUDY" {
		t.Fatalf("QueryRetrieveLevel = %q, want STUDY", level)
	}
}

func TestDefaultCMoveRequest_EmptyUID(t *testing.T) {
	obj := DefaultCMoveRequest("")
	uid := obj.GetString(tags.StudyInstanceUID)
	if uid != "" {
		t.Fatalf("StudyInstanceUID should be empty, got %q", uid)
	}
}

func TestGenerateCFindRequest(t *testing.T) {
	obj := GenerateCFindRequest()
	if obj == nil {
		t.Fatal("GenerateCFindRequest() returned nil")
	}
	if obj.TagCount() == 0 {
		t.Fatal("GenerateCFindRequest() returned empty object")
	}
	patName := obj.GetString(tags.PatientName)
	if patName != "FAKE^PATIENT" {
		t.Fatalf("PatientName = %q, want FAKE^PATIENT", patName)
	}
	modality := obj.GetString(tags.ModalitiesInStudy)
	if modality != "MR" {
		t.Fatalf("ModalitiesInStudy = %q, want MR", modality)
	}
	// UID must be non-empty and at most 64 chars
	uid := obj.GetString(tags.StudyInstanceUID)
	if uid == "" {
		t.Fatal("StudyInstanceUID should be non-empty")
	}
	if len(uid) > 64 {
		t.Fatalf("StudyInstanceUID exceeds 64 chars: len=%d", len(uid))
	}
}
