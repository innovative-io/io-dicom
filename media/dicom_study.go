package media

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

// GetInfo gets information
func (study *DICOMStudy) GetStudy(obj DICOMObject) {
	for i := 0; i < len(obj.GetTags()); i++ {
		tag := obj.GetTagAt(i)
		switch tag.Group {
		case 0x08:
			switch tag.Element {
			case 0x20:
				study.StudyDate = tag.GetString()
			case 0x30:
				study.StudyTime = tag.GetString()
			case 0x50:
				study.AccessionNumber = tag.GetString()
			case 0x60:
				study.Modality = tag.GetString()
			case 0x80:
				study.InstitutionName = tag.GetString()
			case 0x90:
				study.ReferringPhysician = tag.GetString()
			case 0x1030:
				study.Description = tag.GetString()
			}
		case 0x10:
			switch tag.Element {
			case 0x0010:
				study.PatientName = tag.GetString()
			case 0x0020:
				study.PatientID = tag.GetString()
			case 0x0030: //Patient Birth Date
				study.PatientBirthDate = tag.GetString()
			case 0x0040:
				study.PatientSex = tag.GetString()
			}
		case 0x20:
			switch tag.Element {
			case 0x000D:
				study.StudyInstanceUID = tag.GetString()
			}
		}
	}
}
