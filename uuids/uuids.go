package uuids

import (
	"hash/fnv"
	"strconv"

	"github.com/innovative-io/io-dicom/implementation"
)

func hash32(text string) uint32 {
	algorithm := fnv.New32a()
	algorithm.Write([]byte(text))
	return algorithm.Sum32()
}

func CreateStudyUID(patName string, patID string, accNum string, stDate string) string {
	StudyUID := implementation.GetImplementationClassUID()
	value := int(hash32(patName + patID + accNum + stDate))
	uid := StudyUID + "." + strconv.Itoa(value)
	// DICOM UIDs must not exceed 64 characters
	if len(uid) > 64 {
		uid = uid[:64]
	}
	return uid
}

func CreateSeriesUID(RootUID string, Modality string, SeriesNumber string) string {
	value := int(hash32(Modality + SeriesNumber))
	return (RootUID + "." + strconv.Itoa(value)) // 36 bytes + 11 bytes
}

func CreateInstanceUID(RootUID string, InstNumber string) string {
	return (RootUID + "." + InstNumber) // 47 bytes + 2 bytes
}
