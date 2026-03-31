package database

import "github.com/innovative-io/io-dicom/media"

type Database interface {
	AddPatient(dicomObject media.DICOMObject) error
	AddStudy(dicomObject media.DICOMObject) error
	AddSeries(dicomObject media.DICOMObject) error
	AddInstance(dicomObject media.DICOMObject) error
	AddDicom(dicomObject media.DICOMObject) error
}
