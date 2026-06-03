package dimse

import (
	"time"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/internal/uuids"
	"github.com/innovative-io/io-dicom/media"
)

const queryRetrieveLevelStudy = "STUDY"

// writeStudyQueryBase writes the common matching keys shared by C-FIND and C-MOVE study-level requests.
func writeStudyQueryBase(query media.DICOMObject, studyUID string) {
	query.Write(tags.StudyDate, "")
	query.Write(tags.StudyTime, "")
	query.Write(tags.AccessionNumber, "")
	query.Write(tags.QueryRetrieveLevel, queryRetrieveLevelStudy)
	query.Write(tags.ModalitiesInStudy, "")
	query.Write(tags.StudyDescription, "")
	query.Write(tags.PatientName, "")
	query.Write(tags.PatientID, "")
	query.Write(tags.PatientBirthDate, "")
	query.Write(tags.PatientSex, "")
	query.Write(tags.StudyInstanceUID, studyUID)
}

// DefaultCFindRequest creates a default C-FIND request at the STUDY level.
func DefaultCFindRequest() media.DICOMObject {
	query := media.NewEmptyDCMObj()
	writeStudyQueryBase(query, "")
	query.Write(tags.StudyID, "")
	query.Write(tags.NumberOfStudyRelatedSeries, "")
	query.Write(tags.NumberOfStudyRelatedInstances, "")
	return query
}

// DefaultCMoveRequest creates a default C-MOVE request for the given study UID.
func DefaultCMoveRequest(studyUID string) media.DICOMObject {
	query := media.NewEmptyDCMObj()
	writeStudyQueryBase(query, studyUID)
	return query
}

// GenerateCFindRequest builds a populated C-FIND request with synthetic patient data.
func GenerateCFindRequest() media.DICOMObject {
	studyUID := uuids.CreateStudyUID("FAKE^PATIENT", "123456789", "AC1234", time.Now().Format("20060102"))
	query := media.NewEmptyDCMObj()
	query.Write(tags.StudyDate, time.Now())
	query.Write(tags.StudyTime, time.Now())
	query.Write(tags.AccessionNumber, "AC1234")
	query.Write(tags.QueryRetrieveLevel, queryRetrieveLevelStudy)
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
