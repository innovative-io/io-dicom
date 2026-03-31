package media

import "github.com/innovative-io/io-dicom/dictionary/tags"

// DICOMStudy study information structure
type DICOMStudy struct {
	PatientID          string
	PatientName        string
	PatientBirthDate   string
	PatientSex         string
	ReferringPhysician string
	StudyDate          string
	StudyTime          string
	ReportDate         string
	ReportTime         string
	AccessionNumber    string
	Modality           string
	InstitutionName    string
	Description        string
	StudyInstanceUID   string
	ReportText         string
	ObserverName       string
}

func getTagString(obj DICOMObject, tag *tags.Tag) string {
	if t := obj.GetTag(tag); t != nil {
		return t.GetString()
	}
	return ""
}

// GetStudy populates study fields from the DICOM object.
func (study *DICOMStudy) GetStudy(obj DICOMObject) {
	study.StudyDate = getTagString(obj, tags.StudyDate)
	study.StudyTime = getTagString(obj, tags.StudyTime)
	study.AccessionNumber = getTagString(obj, tags.AccessionNumber)
	study.Modality = getTagString(obj, tags.Modality)
	study.InstitutionName = getTagString(obj, tags.InstitutionName)
	study.ReferringPhysician = getTagString(obj, tags.ReferringPhysicianName)
	study.Description = getTagString(obj, tags.StudyDescription)
	study.PatientName = getTagString(obj, tags.PatientName)
	study.PatientID = getTagString(obj, tags.PatientID)
	study.PatientBirthDate = getTagString(obj, tags.PatientBirthDate)
	study.PatientSex = getTagString(obj, tags.PatientSex)
	study.StudyInstanceUID = getTagString(obj, tags.StudyInstanceUID)
}
