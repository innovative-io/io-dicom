package media

import (
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/tags"
)

func TestDICOMStudy_GetStudy(t *testing.T) {
	obj := NewEmptyDCMObj()
	obj.WriteString(tags.PatientName, "SMITH^JANE")
	obj.WriteString(tags.PatientID, "999")
	obj.WriteString(tags.PatientBirthDate, "19800101")
	obj.WriteString(tags.PatientSex, "F")
	obj.WriteString(tags.StudyDate, "20230601")
	obj.WriteString(tags.StudyTime, "120000")
	obj.WriteString(tags.AccessionNumber, "ACC001")
	obj.WriteString(tags.StudyInstanceUID, "1.2.3.4")
	obj.WriteString(tags.StudyDescription, "Chest CT")

	var study DICOMStudy
	study.GetStudy(obj)

	if study.PatientName != "SMITH^JANE" {
		t.Errorf("PatientName = %q", study.PatientName)
	}
	if study.PatientID != "999" {
		t.Errorf("PatientID = %q", study.PatientID)
	}
	if study.StudyDate != "20230601" {
		t.Errorf("StudyDate = %q", study.StudyDate)
	}
	if study.AccessionNumber != "ACC001" {
		t.Errorf("AccessionNumber = %q", study.AccessionNumber)
	}
	if study.StudyInstanceUID != "1.2.3.4" {
		t.Errorf("StudyInstanceUID = %q", study.StudyInstanceUID)
	}
	if study.Description != "Chest CT" {
		t.Errorf("Description = %q", study.Description)
	}
}

func TestDICOMStudy_GetStudy_EmptyObj(t *testing.T) {
	obj := NewEmptyDCMObj()
	var study DICOMStudy
	study.GetStudy(obj)
	if study.PatientName != "" || study.StudyDate != "" {
		t.Error("GetStudy() on empty obj should leave all fields empty")
	}
}

func TestNewJSONObj(t *testing.T) {
	obj := NewJSONObj()
	if obj == nil {
		t.Fatal("NewJSONObj() returned nil")
	}
}

func TestNewJSONObjFromDcmObj(t *testing.T) {
	dcm := NewEmptyDCMObj()
	dcm.WriteString(tags.PatientName, "TEST^PATIENT")
	obj := NewJSONObjFromDcmObj(dcm)
	if obj == nil {
		t.Fatal("NewJSONObjFromDcmObj() returned nil")
	}
}
