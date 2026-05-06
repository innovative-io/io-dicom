package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
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
	cmwl := flag.Bool("mwl", false, "Send a Modality Worklist C-Find request to the destination")
	cmove := flag.Bool("cmove", false, "Send C-Move request to the destination")
	cstore := flag.Bool("cstore", false, "Sends a C-Store request to the destination")

	dump := flag.Bool("dump", false, "Dump contents of DICOM file to stdout")

	datastore := flag.String("datastore", "", "Directory to use as SCP storage")
	healthPort := flag.Int("healthport", 18040, "Dedicated TCP health probe port when running -scp (set 0 to disable)")

	startSCP := flag.Bool("scp", false, "Start a SCP")

	// TLS flags — apply to both the SCP listener and SCU outbound connections.
	tlsEnabled := flag.Bool("tls", false, "Enable TLS. For SCP: requires -tlscert and -tlskey. For SCU: uses system cert pool unless -tlsca is set")
	tlsCert := flag.String("tlscert", "", "Path to TLS certificate file (PEM) — required for -scp -tls")
	tlsKey := flag.String("tlskey", "", "Path to TLS private key file (PEM) — required for -scp -tls")
	tlsCA := flag.String("tlsca", "", "Path to CA certificate file (PEM) used to verify the remote peer")
	tlsInsecure := flag.Bool("tlsinsecure", false, "Skip TLS certificate verification (SCU only — do not use in production)")
	verbose := flag.Bool("verbose", false, "Enable verbose debug logging")

	flag.Parse()

	if *verbose {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}

	if *startSCP {
		if *datastore == "" {
			log.Fatalln("datastore is required for scp")
		}

		if *calledAE == "" {
			log.Fatalln("calledae is required for scp")
		}

		var scp services.SCP
		if *tlsEnabled {
			if *tlsCert == "" || *tlsKey == "" {
				log.Fatalln("-tlscert and -tlskey are required when -scp -tls is set")
			}
			cert, err := tls.LoadX509KeyPair(*tlsCert, *tlsKey)
			if err != nil {
				log.Fatalf("failed to load TLS certificate: %v", err)
			}
			tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
			if *tlsCA != "" {
				pool, err := loadCertPool(*tlsCA)
				if err != nil {
					log.Fatalf("failed to load CA certificate: %v", err)
				}
				tlsCfg.ClientCAs = pool
				tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
			}
			scp = services.NewSCPWithTLS(*port, tlsCfg)
		} else {
			scp = services.NewSCP(*port)
		}

		scp.OnAssociationRequest(func(request network.AssociationRequest) bool {
			called := request.GetCalledAE()
			return *calledAE == called
		})

		scp.OnCFindRequest(func(ctx context.Context, request network.AssociationRequest, queryLevel string, query media.DICOMObject, emit func(media.DICOMObject)) (services.CFindResult, error) {
			query.DumpTags(os.Stdout)
			for i := 0; i < 10; i++ {
				emit(utils.GenerateCFindRequest())
			}
			return services.CFindResult{Status: dicomstatus.Success}, nil
		})

		scp.OnCMoveRequest(func(ctx context.Context, request network.AssociationRequest, moveDestAE string, moveLevel string, query media.DICOMObject, emit func(services.CMoveProgress)) (services.CMoveResult, error) {
			query.DumpTags(os.Stdout)
			return services.CMoveResult{Status: dicomstatus.Success}, nil
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

		var wg sync.WaitGroup
		var healthListener net.Listener
		if *healthPort > 0 {
			var healthErr error
			healthListener, healthErr = startTCPHealthListener(ctx, *healthPort, &wg)
			if healthErr != nil {
				log.Fatalf("failed to start TCP health listener on port %d: %v", *healthPort, healthErr)
			}
			defer healthListener.Close()
		}

		if err := scp.Start(ctx); err != nil {
			log.Fatal(err)
		}
		wg.Wait()
		return
	}

	destination := &network.Destination{
		Name:      *hostName,
		HostName:  *hostName,
		CalledAE:  *calledAE,
		CallingAE: *callingAE,
		Port:      *port,
		IsTLS:     *tlsEnabled,
	}
	if *tlsEnabled {
		tlsCfg := &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: *tlsInsecure, //nolint:gosec
		}
		if *tlsCA != "" {
			pool, err := loadCertPool(*tlsCA)
			if err != nil {
				log.Fatalf("failed to load CA certificate: %v", err)
			}
			tlsCfg.RootCAs = pool
		}
		if *tlsCert != "" && *tlsKey != "" {
			cert, err := tls.LoadX509KeyPair(*tlsCert, *tlsKey)
			if err != nil {
				log.Fatalf("failed to load client TLS certificate: %v", err)
			}
			tlsCfg.Certificates = []tls.Certificate{cert}
		}
		destination.TLSConfig = tlsCfg
	}
	_ = tlsCA // consumed above; suppress unused-variable warning when TLS is off

	if *cecho {
		scu := services.NewSCU(destination)
		err := scu.EchoSCU(context.Background(), 30)
		if err != nil {
			log.Fatalln(err)
		}
		slog.Info("CEcho was successful")
	}
	if *cfind {
		request := utils.DefaultCFindRequest()
		scu := services.NewSCU(destination)
		scu.SetOnCFindResult(func(result media.DICOMObject) {
			log.Printf("Found study %s\n", result.GetString(tags.StudyInstanceUID))
			result.DumpTags(os.Stdout)
		})

		count, status, err := scu.FindSCU(context.Background(), request, 0)
		if err != nil {
			log.Fatalln(err)
		}

		log.Println("CFind was successful")
		log.Printf("Found %d results with status %d\n\n", count, status)
		return
	}
	if *cmwl {
		request := media.NewEmptyDCMObj()
		// Add standard MWL matching keys with empty values (wildcard = match all).
		request.Write(tags.PatientName, "")
		request.Write(tags.PatientID, "")
		request.Write(tags.AccessionNumber, "")
		request.Write(tags.RequestedProcedureID, "")
		scu := services.NewSCU(destination)
		scu.SetOnCFindResult(func(result media.DICOMObject) {
			log.Printf("Worklist item: patient=%s accession=%s\n",
				result.GetString(tags.PatientID), result.GetString(tags.AccessionNumber))
			result.DumpTags(os.Stdout)
		})

		count, status, err := scu.WorklistSCU(context.Background(), request, 0)
		if err != nil {
			log.Fatalln(err)
		}

		log.Println("MWL query was successful")
		log.Printf("Found %d worklist items with status %d\n\n", count, status)
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
		_, err := scu.MoveSCU(context.Background(), *destinationAE, request, 0)
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
		err := scu.StoreSCU(context.Background(), *fileName, 0)
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
		obj.DumpTags(os.Stdout)
		return
	}
}

// loadCertPool reads a PEM-encoded CA certificate file and returns a cert pool
// containing it. Used for both server-side ClientCAs and client-side RootCAs.
func loadCertPool(caFile string) (*x509.CertPool, error) {
	pem, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("no valid certificates found in %s", caFile)
	}
	return pool, nil
}

func startTCPHealthListener(ctx context.Context, port int, wg *sync.WaitGroup) (net.Listener, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}

	log.Printf("INFO: TCP health listener started on port %d", port)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer log.Printf("INFO: TCP health listener stopped on port %d", port)
		for {
			conn, err := listener.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
					return
				}
				log.Printf("WARN: TCP health listener accept failed on port %d: %v", port, err)
				continue
			}
			_ = conn.Close()
		}
	}()

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	return listener, nil
}
