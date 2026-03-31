package main

import (
	"encoding/binary"
	"flag"
	"log"
	"reflect"

	"github.com/innovative-io/io-dicom/media"
)

var version string

func main() {
	log.Printf("Starting compare %s\n\n", version)

	media.InitDict()

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
	srcDicom.DumpTags()

	dstDicom, err := media.NewDCMObjFromFile(*destinationFile)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Dumping destination tags")
	dstDicom.DumpTags()

	compare(srcDicom, dstDicom)
}

func compare(source media.DICOMObject, destination media.DICOMObject) {
	for _, st := range source.GetTags() {
		found := false
		if st.VR == "SQ" {
			sSeq := st.ReadSeq(source.IsExplicitVR())
			dt := destination.GetTagGE(st.Group, st.Element)
			if dt == nil {
				log.Printf("Sequence: (%04X,%04X) %s not found in destination", st.Group, st.Element, st.Name)
				continue
			}
			dSeq := dt.ReadSeq(destination.IsExplicitVR())
			compareSeq(1, sSeq, dSeq)
			continue
		}
		for _, dt := range destination.GetTags() {
			if dt.VR == "SQ" {
				continue
			}
			if st.Group == dt.Group && st.Element == dt.Element {
				found = true
				if !reflect.DeepEqual(st.Data, dt.Data) {
					if len(st.Data) > 128 || len(dt.Data) > 128 {
						log.Printf("Tag: (%04X,%04X) %s are not equal", st.Group, st.Element, st.Name)
					} else {
						switch st.VR {
						case "US":
							if len(st.Data) >= 2 && len(dt.Data) >= 2 {
								log.Printf("Tag: (%04X,%04X) %s are not equal, source: %d, destination: %d", st.Group, st.Element, st.Name, binary.LittleEndian.Uint16(st.Data), binary.LittleEndian.Uint16(dt.Data))
							} else {
								log.Printf("Tag: (%04X,%04X) %s are not equal", st.Group, st.Element, st.Name)
							}
						default:
							log.Printf("Tag: (%04X,%04X) %s are not equal, source: %s, destination: %s", st.Group, st.Element, st.Name, st.Data, dt.Data)
						}
					}
				}
				break
			}
		}
		if !found {
			log.Printf("Tag: (%04X,%04X) %s not found in destination", st.Group, st.Element, st.Name)
		}
	}
}

func compareSeq(indent int, source media.DICOMObject, destination media.DICOMObject) {
	tabs := "\t"
	for i := 0; i < indent; i++ {
		tabs += "\t"
	}

	for _, st := range source.GetTags() {
		found := false
		if st.VR == "SQ" {
			sSeq := st.ReadSeq(source.IsExplicitVR())
			dt := destination.GetTagGE(st.Group, st.Element)
			if dt == nil {
				log.Printf("%sSequence: (%04X,%04X) %s not found in destination", tabs, st.Group, st.Element, st.Name)
				continue
			}
			dSeq := dt.ReadSeq(destination.IsExplicitVR())
			compareSeq(indent+1, sSeq, dSeq)
			continue
		}
		for _, dt := range destination.GetTags() {
			if dt.VR == "SQ" {
				continue
			}
			if st.Group == dt.Group && st.Element == dt.Element {
				found = true
				if !reflect.DeepEqual(st.Data, dt.Data) {
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
				break
			}
		}
		if !found {
			log.Printf("%sTag: (%04X,%04X) %s not found in destination", tabs, st.Group, st.Element, st.Name)
		}
	}
}
