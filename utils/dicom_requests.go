package utils

import (
	"time"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/uuids"
)

// DefaultCFindRequest - Creates a default C-Find request
func DefaultCFindRequest() media.DICOMObject {
	query := media.NewEmptyDCMObj()
	query.WriteString(tags.StudyDate, "")
	query.WriteString(tags.StudyTime, "")
	query.WriteString(tags.AccessionNumber, "")
	query.WriteString(tags.QueryRetrieveLevel, "STUDY")
	query.WriteString(tags.ModalitiesInStudy, "")
	query.WriteString(tags.StudyDescription, "")
	query.WriteString(tags.PatientName, "")
	query.WriteString(tags.PatientID, "")
	query.WriteString(tags.PatientBirthDate, "")
	query.WriteString(tags.PatientSex, "")
	query.WriteString(tags.StudyInstanceUID, "")
	query.WriteString(tags.StudyID, "")
	query.WriteString(tags.NumberOfStudyRelatedSeries, "")
	query.WriteString(tags.NumberOfStudyRelatedInstances, "")
	return query
}

// DefaultCMoveRequest - Creates a default C-Move request
func DefaultCMoveRequest(studyUID string) media.DICOMObject {
	query := media.NewEmptyDCMObj()
	query.WriteString(tags.StudyDate, "")
	query.WriteString(tags.StudyTime, "")
	query.WriteString(tags.AccessionNumber, "")
	query.WriteString(tags.QueryRetrieveLevel, "STUDY")
	query.WriteString(tags.ModalitiesInStudy, "")
	query.WriteString(tags.StudyDescription, "")
	query.WriteString(tags.PatientName, "")
	query.WriteString(tags.PatientID, "")
	query.WriteString(tags.PatientBirthDate, "")
	query.WriteString(tags.PatientSex, "")
	query.WriteString(tags.StudyInstanceUID, studyUID)
	return query
}

// GenerateCFindRequest - Generates C-Find request
func GenerateCFindRequest() media.DICOMObject {
	studyUID := uuids.CreateStudyUID("FAKE^PATIENT", "123456789", "AC1234", time.Now().Format("20060102"))
	query := media.NewEmptyDCMObj()
	query.WriteDate(tags.StudyDate, time.Now())
	query.WriteTime(tags.StudyTime, time.Now())
	query.WriteString(tags.AccessionNumber, "AC1234")
	query.WriteString(tags.QueryRetrieveLevel, "STUDY")
	query.WriteString(tags.ModalitiesInStudy, "MR")
	query.WriteString(tags.StudyDescription, "Fake study")
	query.WriteString(tags.PatientName, "FAKE^PATIENT")
	query.WriteString(tags.PatientID, "123456789")
	query.WriteDate(tags.PatientBirthDate, time.Now())
	query.WriteString(tags.PatientSex, "O")
	query.WriteString(tags.StudyInstanceUID, studyUID)
	query.WriteString(tags.StudyID, "123")
	query.WriteString(tags.NumberOfStudyRelatedSeries, "1")
	query.WriteString(tags.NumberOfStudyRelatedInstances, "1")
	return query
}
