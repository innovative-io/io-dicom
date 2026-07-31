package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/dimse"
	"github.com/innovative-io/io-dicom/media"
	"github.com/innovative-io/io-dicom/network"
	"github.com/innovative-io/io-dicom/network/dicomstatus"
	"github.com/innovative-io/io-dicom/services"
)

// safeComponent reduces one wire-supplied attribute to a single, inert path
// element. Anything outside a conservative allow-list becomes '_', so a
// separator or a traversal sequence cannot survive.
func safeComponent(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9',
			r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	// "", ".", ".." and friends are all path-significant rather than names.
	if strings.Trim(out, ".") == "" {
		return ""
	}
	if len(out) > 64 {
		out = out[:64]
	}
	return out
}

// storagePath builds the on-disk path for a received instance and guarantees it
// stays inside root.
//
// PatientID, StudyInstanceUID, SeriesInstanceUID and SOPInstanceUID all arrive
// in the C-STORE payload from the remote peer. They were passed straight to
// filepath.Join, which cleans a path but does not contain it: Join("/data",
// "../../etc") is "/etc". A peer needed only to send PatientID "../../.." to
// write a .dcm file anywhere this process could write, with no authentication.
// PatientID is an LO, not a UID, so it carries arbitrary text by design.
//
// Each component is sanitised, and the assembled path is then re-checked
// against root so that any future change to the sanitiser still cannot escape.
func storagePath(root, patientID, studyUID, seriesUID, sopUID string) (string, error) {
	parts := map[string]string{
		"PatientID":         patientID,
		"StudyInstanceUID":  studyUID,
		"SeriesInstanceUID": seriesUID,
		"SOPInstanceUID":    sopUID,
	}
	clean := make(map[string]string, len(parts))
	for name, raw := range parts {
		c := safeComponent(raw)
		if c == "" {
			return "", fmt.Errorf("unusable %s in received instance", name)
		}
		clean[name] = c
	}

	path := filepath.Join(root,
		clean["PatientID"], clean["StudyInstanceUID"], clean["SeriesInstanceUID"],
		clean["SOPInstanceUID"]+".dcm")

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("refusing to write outside the datastore")
	}
	return path, nil
}

var version string

// fatal logs msg at error level with the given structured attributes and exits
// the process with a non-zero status. It replaces log.Fatal* so the CLI emits
// the same structured slog output as the rest of the library.
func fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}

func main() {
	slog.Info("starting io-dicom", "version", version)

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
			fatal("datastore is required for scp")
		}

		if *calledAE == "" {
			fatal("calledae is required for scp")
		}

		var scp services.SCP
		if *tlsEnabled {
			if *tlsCert == "" || *tlsKey == "" {
				fatal("-tlscert and -tlskey are required when -scp -tls is set")
			}
			cert, err := tls.LoadX509KeyPair(*tlsCert, *tlsKey)
			if err != nil {
				fatal("failed to load TLS certificate", "error", err)
			}
			tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
			if *tlsCA != "" {
				pool, err := loadCertPool(*tlsCA)
				if err != nil {
					fatal("failed to load CA certificate", "error", err)
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
				emit(dimse.GenerateCFindRequest())
			}
			return services.CFindResult{Status: dicomstatus.Success}, nil
		})

		scp.OnCMoveRequest(func(ctx context.Context, request network.AssociationRequest, moveDestAE string, moveLevel string, query media.DICOMObject, emit func(services.CMoveProgress)) (services.CMoveResult, error) {
			query.DumpTags(os.Stdout)
			return services.CMoveResult{Status: dicomstatus.Success}, nil
		})

		scp.OnCStoreRequest(func(ctx context.Context, request network.AssociationRequest, data media.DICOMObject) uint16 {
			slog.Info("C-Store received", "sop_instance_uid", data.GetString(tags.SOPInstanceUID))
			path, err := storagePath(*datastore,
				data.GetString(tags.PatientID),
				data.GetString(tags.StudyInstanceUID),
				data.GetString(tags.SeriesInstanceUID),
				data.GetString(tags.SOPInstanceUID))
			if err != nil {
				slog.Error("refusing to store instance", "error", err)
				return dicomstatus.FailureProcessingFailure
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				slog.Error("failed to create storage directory", "path", filepath.Dir(path), "error", err)
				return dicomstatus.FailureProcessingFailure
			}

			if err := data.WriteToFile(path); err != nil {
				slog.Error("failed to save instance", "path", path, "error", err)
				return dicomstatus.FailureProcessingFailure
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
				fatal("failed to start TCP health listener", "port", *healthPort, "error", healthErr)
			}
			defer healthListener.Close()
		}

		if err := scp.Start(ctx); err != nil {
			fatal("scp failed", "error", err)
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
				fatal("failed to load CA certificate", "error", err)
			}
			tlsCfg.RootCAs = pool
		}
		if *tlsCert != "" && *tlsKey != "" {
			cert, err := tls.LoadX509KeyPair(*tlsCert, *tlsKey)
			if err != nil {
				fatal("failed to load client TLS certificate", "error", err)
			}
			tlsCfg.Certificates = []tls.Certificate{cert}
		}
		destination.TLSConfig = tlsCfg
	}
	_ = tlsCA // consumed above; suppress unused-variable warning when TLS is off

	if *cecho {
		scu := services.NewSCU(destination)
		err := scu.EchoSCU(context.Background())
		if err != nil {
			fatal("C-Echo failed", "error", err)
		}
		slog.Info("C-Echo successful")
	}
	if *cfind {
		request := dimse.DefaultCFindRequest()
		scu := services.NewSCU(destination)
		scu.SetOnCFindResult(func(result media.DICOMObject) {
			slog.Info("found study", "study_instance_uid", result.GetString(tags.StudyInstanceUID))
			result.DumpTags(os.Stdout)
		})

		count, status, err := scu.FindSCU(context.Background(), request)
		if err != nil {
			fatal("C-Find failed", "error", err)
		}

		slog.Info("C-Find successful", "results", count, "status", status)
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
			slog.Info("worklist item",
				"patient_id", result.GetString(tags.PatientID), "accession_number", result.GetString(tags.AccessionNumber))
			result.DumpTags(os.Stdout)
		})

		count, status, err := scu.WorklistSCU(context.Background(), request)
		if err != nil {
			fatal("MWL query failed", "error", err)
		}

		slog.Info("MWL query successful", "items", count, "status", status)
		return
	}
	if *cmove {
		if *destinationAE == "" {
			fatal("destinationae is required for a C-Move")
		}
		if *studyUID == "" {
			fatal("studyuid is required for a C-Move")
		}

		request := dimse.DefaultCMoveRequest(*studyUID)

		scu := services.NewSCU(destination)
		_, err := scu.MoveSCU(context.Background(), *destinationAE, request)
		if err != nil {
			fatal("C-Move failed", "error", err)
		}
		slog.Info("C-Move successful")
		return
	}
	if *cstore {
		if *fileName == "" {
			fatal("file is required for a C-Store")
		}
		scu := services.NewSCU(destination)
		err := scu.StoreSCU(context.Background(), *fileName)
		if err != nil {
			fatal("C-Store failed", "error", err)
		}
		slog.Info("C-Store successful", "file", *fileName)
		return
	}
	if *dump {
		if *fileName == "" {
			fatal("file is required for a dump")
		}
		obj, err := media.NewDCMObjFromFile(*fileName)
		if err != nil {
			fatal("failed to read DICOM file", "error", err)
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

	slog.Info("TCP health listener started", "port", port)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer slog.Info("TCP health listener stopped", "port", port)
		for {
			conn, err := listener.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
					return
				}
				slog.Warn("TCP health listener accept failed", "port", port, "error", err)
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
