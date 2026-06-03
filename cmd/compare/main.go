package main

import (
	"encoding/binary"
	"flag"
	"log"
	"os"
	"reflect"
	"strings"

	"github.com/innovative-io/io-dicom/media"
)

var version string

func main() {
	log.Printf("Starting compare %s\n\n", version)

	sourceFile := flag.String("s", "", "Source DICOM file")
	destinationFile := flag.String("d", "", "Destination DICOM file to compare against")

	flag.Parse()

	if *sourceFile == "" || *destinationFile == "" {
		log.Fatalln("Both a source and destination is required")
	}
	srcDicom, err := media.NewDCMObjFromFile(*sourceFile)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Dumping source tags")
	srcDicom.DumpTags(os.Stdout)

	dstDicom, err := media.NewDCMObjFromFile(*destinationFile)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Dumping destination tags")
	dstDicom.DumpTags(os.Stdout)

	if compare(srcDicom, dstDicom) {
		os.Exit(1)
	}
}

// compare returns true if any differences are found between source and destination.
func compare(source media.DICOMObject, destination media.DICOMObject) bool {
	return compareSeq(0, source, destination)
}

// compareSeq compares two DICOM objects at a given nesting depth.
// indent controls the tab prefix for log output; 0 means no prefix.
func compareSeq(indent int, source media.DICOMObject, destination media.DICOMObject) bool {
	hasDiff := false
	tabs := strings.Repeat("\t", indent)

	// Build O(1) lookup map for all destination tags (including SQ).
	destMap := make(map[uint32]*media.DICOMTag)
	for _, dt := range destination.GetTags() {
		key := uint32(dt.Group)<<16 | uint32(dt.Element)
		destMap[key] = dt
	}

	for _, st := range source.GetTags() {
		if st.VR == "SQ" {
			sSeq := st.ReadSeq(source.IsExplicitVR())
			key := uint32(st.Group)<<16 | uint32(st.Element)
			dt := destMap[key]
			if dt == nil {
				log.Printf("%sSequence: (%04X,%04X) %s not found in destination", tabs, st.Group, st.Element, st.Name)
				hasDiff = true
				continue
			}
			dSeq := dt.ReadSeq(destination.IsExplicitVR())
			if compareSeq(indent+1, sSeq, dSeq) {
				hasDiff = true
			}
			continue
		}
		key := uint32(st.Group)<<16 | uint32(st.Element)
		dt, found := destMap[key]
		if !found {
			log.Printf("%sTag: (%04X,%04X) %s not found in destination", tabs, st.Group, st.Element, st.Name)
			hasDiff = true
			continue
		}
		if !reflect.DeepEqual(st.Data, dt.Data) {
			hasDiff = true
			if len(st.Data) > 128 || len(dt.Data) > 128 {
				log.Printf("%sTag: (%04X,%04X) %s are not equal", tabs, st.Group, st.Element, st.Name)
			} else {
				switch st.VR {
				case "US":
					if len(st.Data) >= 2 && len(dt.Data) >= 2 {
						log.Printf("%sTag: (%04X,%04X) %s are not equal, source: %d, destination: %d", tabs, st.Group, st.Element, st.Name, binary.LittleEndian.Uint16(st.Data), binary.LittleEndian.Uint16(dt.Data))
					} else {
						log.Printf("%sTag: (%04X,%04X) %s are not equal", tabs, st.Group, st.Element, st.Name)
					}
				default:
					log.Printf("%sTag: (%04X,%04X) %s are not equal, source: %s, destination: %s", tabs, st.Group, st.Element, st.Name, st.Data, dt.Data)
				}
			}
		}
	}
	return hasDiff
}
