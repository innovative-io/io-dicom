package media

import (
	"time"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/uuids"
)

// DefaultCFindRequest - Creates a default C-Find request
func DefaultCFindRequest() DICOMObject {
	query := NewEmptyDCMObj()
	query.Write(tags.StudyDate, "")
	query.Write(tags.StudyTime, "")
	query.Write(tags.AccessionNumber, "")
	query.Write(tags.QueryRetrieveLevel, "STUDY")
	query.Write(tags.ModalitiesInStudy, "")
	query.Write(tags.StudyDescription, "")
	query.Write(tags.PatientName, "")
	query.Write(tags.PatientID, "")
	query.Write(tags.PatientBirthDate, "")
	query.Write(tags.PatientSex, "")
	query.Write(tags.StudyInstanceUID, "")
	query.Write(tags.StudyID, "")
	query.Write(tags.NumberOfStudyRelatedSeries, "")
	query.Write(tags.NumberOfStudyRelatedInstances, "")
	return query
}

// DefaultCMoveRequest - Creates a default C-Move request
func DefaultCMoveRequest(studyUID string) DICOMObject {
	query := NewEmptyDCMObj()
	query.Write(tags.StudyDate, "")
	query.Write(tags.StudyTime, "")
	query.Write(tags.AccessionNumber, "")
	query.Write(tags.QueryRetrieveLevel, "STUDY")
	query.Write(tags.ModalitiesInStudy, "")
	query.Write(tags.StudyDescription, "")
	query.Write(tags.PatientName, "")
	query.Write(tags.PatientID, "")
	query.Write(tags.PatientBirthDate, "")
	query.Write(tags.PatientSex, "")
	query.Write(tags.StudyInstanceUID, studyUID)
	return query
}

// GenerateCFindRequest - Generates C-Find request
func GenerateCFindRequest() DICOMObject {
	studyUID := uuids.CreateStudyUID("FAKE^PATIENT", "123456789", "AC1234", time.Now().Format("20060102"))
	query := NewEmptyDCMObj()
	query.Write(tags.StudyDate, time.Now())
	query.Write(tags.StudyTime, time.Now())
	query.Write(tags.AccessionNumber, "AC1234")
	query.Write(tags.QueryRetrieveLevel, "STUDY")
	query.Write(tags.ModalitiesInStudy, "MR")
	query.Write(tags.StudyDescription, "Fake study")
	query.Write(tags.PatientName, "FAKE^PATIENT")
	query.Write(tags.PatientID, "123456789")
	query.Write(tags.PatientBirthDate, time.Now())
	query.Write(tags.PatientSex, "O")
	query.Write(tags.StudyInstanceUID, studyUID)
	query.Write(tags.StudyID, "123")
	query.Write(tags.NumberOfStudyRelatedSeries, "1")
	query.Write(tags.NumberOfStudyRelatedInstances, "1")
	return query
}
