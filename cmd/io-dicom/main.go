package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomstatus"
	"github.com/innovative-io/io-dicom/services"
	"github.com/innovative-io/io-dicom/utils"
)

var version string

func main() {
	log.Printf("Starting io-dicom %s\n\n", version)

	media.InitDict()

	hostName := flag.String("host", "localhost", "Destination host name or IP")
	calledAE := flag.String("calledae", "DICOM_SCP", "AE of the destination")
	callingAE := flag.String("callingae", "DICOM_SCU", "AE of the client")
	port := flag.Int("port", 1040, "Port of the destination system")

	studyUID := flag.String("studyuid", "", "Study UID to be added to request")

	destinationAE := flag.String("destinationae", "", "AE of the destination for a C-Move request")

	fileName := flag.String("file", "", "DICOM file to be sent")

	cecho := flag.Bool("cecho", false, "Send C-Echo to the destination")
	cfind := flag.Bool("cfind", false, "Send C-Find request to the destination")
	cmove := flag.Bool("cmove", false, "Send C-Move request to the destination")
	cstore := flag.Bool("cstore", false, "Sends a C-Store request to the destination")

	dump := flag.Bool("dump", false, "Dump contents of DICOM file to stdout")

	datastore := flag.String("datastore", "", "Directory to use as SCP storage")

	startSCP := flag.Bool("scp", false, "Start a SCP")

	flag.Parse()

	if *startSCP {
		if *datastore == "" {
			log.Fatalln("datastore is required for scp")
		}

		if *calledAE == "" {
			log.Fatalln("calledae is required for scp")
		}
		scp := services.NewSCP(*port)

		scp.OnAssociationRequest(func(request network.AssociationRequest) bool {
			called := request.GetCalledAE()
			return *calledAE == called
		})

		scp.OnCFindRequest(func(request network.AssociationRequest, queryLevel string, query media.DICOMObject) ([]media.DICOMObject, uint16) {
			query.DumpTags()
			results := make([]media.DICOMObject, 0)
			for i := 0; i < 10; i++ {
				results = append(results, utils.GenerateCFindRequest())
			}
			return results, dicomstatus.Success
		})

		scp.OnCMoveRequest(func(request network.AssociationRequest, moveLevel string, query media.DICOMObject) uint16 {
			query.DumpTags()
			return dicomstatus.Success
		})

		scp.OnCStoreRequest(func(request network.AssociationRequest, data media.DICOMObject) uint16 {
			log.Printf("INFO, C-Store received %s", data.GetString(tags.SOPInstanceUID))
			directory := filepath.Join(*datastore, data.GetString(tags.PatientID), data.GetString(tags.StudyInstanceUID), data.GetString(tags.SeriesInstanceUID))
			os.MkdirAll(directory, 0755)

			path := filepath.Join(directory, data.GetString(tags.SOPInstanceUID)+".dcm")

			err := data.WriteToFile(path)
			if err != nil {
				log.Printf("ERROR: There was an error saving %s : %s", path, err.Error())
			}
			return dicomstatus.Success
		})

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := scp.Start(ctx); err != nil {
			log.Fatal(err)
		}
		return
	}

	destination := &network.Destination{
		Name:      *hostName,
		HostName:  *hostName,
		CalledAE:  *calledAE,
		CallingAE: *callingAE,
		Port:      *port,
		IsCFind:   true,
		IsCStore:  true,
		IsMWL:     true,
		IsTLS:     false,
	}

	if *cecho {
		scu := services.NewSCU(destination)
		err := scu.EchoSCU(30)
		if err != nil {
			log.Fatalln(err)
		}
		log.Println("CEcho was successful")
	}
	if *cfind {
		request := utils.DefaultCFindRequest()
		scu := services.NewSCU(destination)
		scu.SetOnCFindResult(func(result media.DICOMObject) {
			log.Printf("Found study %s\n", result.GetString(tags.StudyInstanceUID))
			result.DumpTags()
		})

		count, status, err := scu.FindSCU(request, 0)
		if err != nil {
			log.Fatalln(err)
		}

		log.Println("CFind was successful")
		log.Printf("Found %d results with status %d\n\n", count, status)
		return
	}
	if *cmove {
		if *destinationAE == "" {
			log.Fatalln("destinationae is required for a C-Move")
		}
		if *studyUID == "" {
			log.Fatalln("studyuid is required for a C-Move")
		}

		request := utils.DefaultCMoveRequest(*studyUID)

		scu := services.NewSCU(destination)
		_, err := scu.MoveSCU(*destinationAE, request, 0)
		if err != nil {
			log.Fatalln(err)
		}
		log.Println("CMove was successful")
		return
	}
	if *cstore {
		if *fileName == "" {
			log.Fatalln("file is required for a C-Store")
		}
		scu := services.NewSCU(destination)
		err := scu.StoreSCU(*fileName, 0)
		if err != nil {
			log.Fatalln(err)
		}
		log.Printf("CStore of %s was successful", *fileName)
		return
	}
	if *dump {
		if *fileName == "" {
			log.Fatalln("file is required for a dump")
		}
		obj, err := media.NewDCMObjFromFile(*fileName)
		if err != nil {
			log.Fatal(err)
		}
		obj.DumpTags()
		return
	}
}
