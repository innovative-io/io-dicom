package wado

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/innovative-io/io-dicom/dictionary/tags"
	"github.com/innovative-io/io-dicom/media"
)

// Store is the storage backend required by the WADO server.
// Callers provide their own implementation backed by a database,
// filesystem, or in-memory map.
type Store interface {
	// RetrieveStudy returns all instances belonging to the given study.
	RetrieveStudy(ctx context.Context, studyUID string) ([]media.DICOMObject, error)
	// RetrieveSeries returns all instances belonging to the given series.
	RetrieveSeries(ctx context.Context, studyUID, seriesUID string) ([]media.DICOMObject, error)
	// RetrieveInstance returns the single specified instance.
	RetrieveInstance(ctx context.Context, studyUID, seriesUID, sopInstanceUID string) (media.DICOMObject, error)
	// StoreInstances persists the supplied DICOM objects.
	StoreInstances(ctx context.Context, objects []media.DICOMObject) error
	// SearchStudies returns studies matching the query parameters.
	SearchStudies(ctx context.Context, query url.Values) ([]media.DICOMObject, error)
	// SearchSeries returns series matching the query parameters.
	SearchSeries(ctx context.Context, studyUID string, query url.Values) ([]media.DICOMObject, error)
	// SearchInstances returns instances matching the query parameters.
	SearchInstances(ctx context.Context, studyUID, seriesUID string, query url.Values) ([]media.DICOMObject, error)
	// DeleteStudy removes all instances belonging to a study.
	DeleteStudy(ctx context.Context, studyUID string) error
	// DeleteSeries removes all instances belonging to a series.
	DeleteSeries(ctx context.Context, studyUID, seriesUID string) error
	// DeleteInstance removes a single DICOM instance.
	DeleteInstance(ctx context.Context, studyUID, seriesUID, sopInstanceUID string) error
}

// ServerParams configures a WADO server.
type ServerParams struct {
	// Port is the TCP port to listen on (e.g. 8080).
	Port int
	// Store is the required storage backend.
	Store Store
	// ReadHeaderTimeout caps the time allowed to read request headers.
	// Defaults to 10 s when zero. Set -1 to disable (not recommended for
	// internet-facing deployments).
	ReadHeaderTimeout time.Duration
	// WriteTimeout caps the time from accepting a connection to writing the
	// full response. Defaults to 120 s when zero; -1 disables it.
	WriteTimeout time.Duration
	// IdleTimeout caps the time an idle keep-alive connection is held open.
	// Defaults to 60 s when zero; -1 disables it.
	IdleTimeout time.Duration
}

// Server is the combined WADO-RS / STOW-RS / QIDO-RS HTTP server.
type Server interface {
	// Start begins accepting HTTP connections and blocks until ctx is
	// cancelled or Stop is called.
	Start(ctx context.Context) error
	// Stop gracefully shuts the HTTP server down.
	Stop(ctx context.Context) error
	// Handler returns the underlying http.Handler for embedding in another mux.
	Handler() http.Handler
}

type wadoServer struct {
	params ServerParams
	mux    *http.ServeMux
	srv    *http.Server
}

// resolveTimeout returns d if d is non-zero, defaultVal if d is zero, or 0 if
// d is negative (caller opted out).
func resolveTimeout(d, defaultVal time.Duration) time.Duration {
	if d == 0 {
		return defaultVal
	}
	if d < 0 {
		return 0
	}
	return d
}

// NewServer creates and configures a new WADO server.
func NewServer(params ServerParams) Server {
	s := &wadoServer{params: params}
	s.mux = s.buildMux()
	s.srv = &http.Server{
		Addr:              fmt.Sprintf(":%d", params.Port),
		Handler:           s.mux,
		ReadHeaderTimeout: resolveTimeout(params.ReadHeaderTimeout, 10*time.Second),
		WriteTimeout:      resolveTimeout(params.WriteTimeout, 120*time.Second),
		IdleTimeout:       resolveTimeout(params.IdleTimeout, 60*time.Second),
	}
	return s
}

// Start begins accepting connections. It returns when ctx is cancelled or
// a fatal listener error occurs.
func (s *wadoServer) Start(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		slog.Info("WADO server starting", "addr", s.srv.Addr)
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	select {
	case <-ctx.Done():
		return s.Stop(context.Background())
	case err := <-errCh:
		return err
	}
}

// Stop gracefully shuts down the server.
func (s *wadoServer) Stop(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

// Handler returns the http.Handler for embedding in other servers.
func (s *wadoServer) Handler() http.Handler {
	return s.mux
}

func (s *wadoServer) buildMux() *http.ServeMux {
	mux := http.NewServeMux()

	// WADO-RS retrieve
	mux.HandleFunc("GET /wado/rs/studies/{studyUID}", s.retrieveStudy)
	mux.HandleFunc("GET /wado/rs/studies/{studyUID}/series/{seriesUID}", s.retrieveSeries)
	mux.HandleFunc("GET /wado/rs/studies/{studyUID}/series/{seriesUID}/instances/{sopInstanceUID}", s.retrieveInstance)
	mux.HandleFunc("GET /wado/rs/studies/{studyUID}/metadata", s.retrieveStudyMetadata)
	mux.HandleFunc("GET /wado/rs/studies/{studyUID}/series/{seriesUID}/metadata", s.retrieveSeriesMetadata)
	mux.HandleFunc("GET /wado/rs/studies/{studyUID}/series/{seriesUID}/instances/{sopInstanceUID}/metadata", s.retrieveInstanceMetadata)
	mux.HandleFunc("GET /wado/rs/studies/{studyUID}/series/{seriesUID}/instances/{sopInstanceUID}/frames/{frames}", s.retrieveFrames)

	// STOW-RS store
	mux.HandleFunc("POST /stow/rs/studies", s.storeInstances)
	mux.HandleFunc("POST /stow/rs/studies/{studyUID}", s.storeInstances)

	// QIDO-RS search
	mux.HandleFunc("GET /qido/rs/studies", s.searchStudies)
	mux.HandleFunc("GET /qido/rs/studies/{studyUID}/series", s.searchSeries)
	mux.HandleFunc("GET /qido/rs/studies/{studyUID}/series/{seriesUID}/instances", s.searchInstances)

	// WADO-RS delete
	mux.HandleFunc("DELETE /wado/rs/studies/{studyUID}", s.deleteStudy)
	mux.HandleFunc("DELETE /wado/rs/studies/{studyUID}/series/{seriesUID}", s.deleteSeries)
	mux.HandleFunc("DELETE /wado/rs/studies/{studyUID}/series/{seriesUID}/instances/{sopInstanceUID}", s.deleteInstance)

	return mux
}

// ── WADO-RS retrieve handlers ─────────────────────────────────────────────────

func (s *wadoServer) retrieveStudy(w http.ResponseWriter, r *http.Request) {
	if !requireUIDs(w, r.PathValue("studyUID")) {
		return
	}
	objects, err := s.params.Store.RetrieveStudy(r.Context(), r.PathValue("studyUID"))
	if err != nil {
		httpError(w, err, http.StatusNotFound)
		return
	}
	writeMultipartDICOM(w, objects)
}

func (s *wadoServer) retrieveSeries(w http.ResponseWriter, r *http.Request) {
	if !requireUIDs(w, r.PathValue("studyUID"), r.PathValue("seriesUID")) {
		return
	}
	objects, err := s.params.Store.RetrieveSeries(r.Context(),
		r.PathValue("studyUID"), r.PathValue("seriesUID"))
	if err != nil {
		httpError(w, err, http.StatusNotFound)
		return
	}
	writeMultipartDICOM(w, objects)
}

func (s *wadoServer) retrieveInstance(w http.ResponseWriter, r *http.Request) {
	if !requireUIDs(w, r.PathValue("studyUID"), r.PathValue("seriesUID"), r.PathValue("sopInstanceUID")) {
		return
	}
	obj, err := s.params.Store.RetrieveInstance(r.Context(),
		r.PathValue("studyUID"), r.PathValue("seriesUID"), r.PathValue("sopInstanceUID"))
	if err != nil {
		httpError(w, err, http.StatusNotFound)
		return
	}
	writeMultipartDICOM(w, []media.DICOMObject{obj})
}

func (s *wadoServer) retrieveStudyMetadata(w http.ResponseWriter, r *http.Request) {
	if !requireUIDs(w, r.PathValue("studyUID")) {
		return
	}
	objects, err := s.params.Store.RetrieveStudy(r.Context(), r.PathValue("studyUID"))
	if err != nil {
		httpError(w, err, http.StatusNotFound)
		return
	}
	writeJSONMetadata(w, objects)
}

func (s *wadoServer) retrieveSeriesMetadata(w http.ResponseWriter, r *http.Request) {
	if !requireUIDs(w, r.PathValue("studyUID"), r.PathValue("seriesUID")) {
		return
	}
	objects, err := s.params.Store.RetrieveSeries(r.Context(),
		r.PathValue("studyUID"), r.PathValue("seriesUID"))
	if err != nil {
		httpError(w, err, http.StatusNotFound)
		return
	}
	writeJSONMetadata(w, objects)
}

func (s *wadoServer) retrieveInstanceMetadata(w http.ResponseWriter, r *http.Request) {
	if !requireUIDs(w, r.PathValue("studyUID"), r.PathValue("seriesUID"), r.PathValue("sopInstanceUID")) {
		return
	}
	obj, err := s.params.Store.RetrieveInstance(r.Context(),
		r.PathValue("studyUID"), r.PathValue("seriesUID"), r.PathValue("sopInstanceUID"))
	if err != nil {
		httpError(w, err, http.StatusNotFound)
		return
	}
	writeJSONMetadata(w, []media.DICOMObject{obj})
}

// retrieveFrames handles WADO-RS frame-level retrieval.
// The {frames} path segment is a comma-separated list of 1-based frame numbers.
func (s *wadoServer) retrieveFrames(w http.ResponseWriter, r *http.Request) {
	if !requireUIDs(w, r.PathValue("studyUID"), r.PathValue("seriesUID"), r.PathValue("sopInstanceUID")) {
		return
	}
	obj, err := s.params.Store.RetrieveInstance(r.Context(),
		r.PathValue("studyUID"), r.PathValue("seriesUID"), r.PathValue("sopInstanceUID"))
	if err != nil {
		httpError(w, err, http.StatusNotFound)
		return
	}
	frames, err := parseFrameList(r.PathValue("frames"))
	if err != nil {
		http.Error(w, "invalid frame list", http.StatusBadRequest)
		return
	}
	mw := multipart.NewWriter(w)
	w.Header().Set("Content-Type", fmt.Sprintf(
		`multipart/related; type="application/octet-stream"; boundary=%s`, mw.Boundary()))
	for _, f := range frames {
		// GetDecompressedFrame, not GetPixelData: the latter sizes (and caps) its
		// allocation across ALL frames, so any object whose total pixel data
		// exceeds the 512 MiB cap failed on every frame and produced an empty
		// 200 response. It also returns raw compressed fragments for encapsulated
		// syntaxes, whereas WADO-RS octet-stream frames are uncompressed.
		data, pixErr := obj.GetDecompressedFrame(r.Context(), f-1) // convert to 0-based
		if pixErr != nil {
			slog.Warn("wado: failed to read frame", "frame", f, "err", pixErr)
			continue
		}
		part, partErr := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type": []string{"application/octet-stream"},
		})
		if partErr != nil {
			break
		}
		if _, writeErr := part.Write(data); writeErr != nil {
			slog.Warn("wado: failed to write frame", "frame", f, "err", writeErr)
			break
		}
	}
	_ = mw.Close()
}

// ── STOW-RS store handler ─────────────────────────────────────────────────────

func (s *wadoServer) storeInstances(w http.ResponseWriter, r *http.Request) {
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		http.Error(w, "expected multipart/related content", http.StatusBadRequest)
		return
	}
	boundary := params["boundary"]
	if boundary == "" {
		http.Error(w, "missing multipart boundary", http.StatusBadRequest)
		return
	}
	// Bound the request body as a whole; each part is separately capped by
	// limitedReadAll.
	mr := multipart.NewReader(http.MaxBytesReader(w, r.Body, maxStoreBodyBytes), boundary)
	var objects []media.DICOMObject
	failed := 0
	for {
		part, partErr := mr.NextPart()
		if partErr != nil {
			if errors.Is(partErr, io.EOF) {
				break
			}
			// A malformed or over-large body is a client error, not the end of
			// the stream. Treating every NextPart error as "done" is what let a
			// truncated upload report success for the parts that happened to
			// arrive first.
			http.Error(w, "malformed multipart body", http.StatusBadRequest)
			return
		}
		if len(objects)+failed >= maxStoreParts {
			http.Error(w, "too many parts", http.StatusRequestEntityTooLarge)
			return
		}
		data, readErr := limitedReadAll(part)
		if readErr != nil {
			slog.Warn("STOW-RS: failed to read part", "err", readErr)
			failed++
			continue
		}
		obj, parseErr := media.NewDCMObjFromBytes(data)
		if parseErr != nil {
			slog.Warn("STOW-RS: failed to parse DICOM part", "err", parseErr)
			failed++
			continue
		}
		objects = append(objects, obj)
	}

	// PS3.18 §10.5: nothing stored is a failure, not a success. Returning 200
	// with an empty ReferencedSOPSequence told a sending modality its study was
	// safely archived when none of it was — and a modality that then deletes its
	// local copy loses the study.
	if len(objects) == 0 {
		if failed > 0 {
			http.Error(w, "no instances could be stored", http.StatusConflict)
			return
		}
		http.Error(w, "no DICOM instances in request", http.StatusBadRequest)
		return
	}
	if storeErr := s.params.Store.StoreInstances(r.Context(), objects); storeErr != nil {
		httpError(w, storeErr, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/dicom+json")
	// PS3.18 §10.5: partial success is 202 Accepted, so a client can tell that
	// some instances did not make it rather than assuming the whole study landed.
	if failed > 0 {
		slog.Warn("STOW-RS: partial store", "stored", len(objects), "failed", failed)
		w.WriteHeader(http.StatusAccepted)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	_ = json.NewEncoder(w).Encode(buildStowResponse(objects))
}

// ── QIDO-RS search handlers ───────────────────────────────────────────────────

func (s *wadoServer) searchStudies(w http.ResponseWriter, r *http.Request) {
	objects, err := s.params.Store.SearchStudies(r.Context(), r.URL.Query())
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	// Per DICOM PS3.18 §10.6.3.3: respond 204 No Content when there are no matches.
	if len(objects) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSONMetadata(w, objects)
}

func (s *wadoServer) searchSeries(w http.ResponseWriter, r *http.Request) {
	objects, err := s.params.Store.SearchSeries(r.Context(),
		r.PathValue("studyUID"), r.URL.Query())
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	if len(objects) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSONMetadata(w, objects)
}

func (s *wadoServer) searchInstances(w http.ResponseWriter, r *http.Request) {
	objects, err := s.params.Store.SearchInstances(r.Context(),
		r.PathValue("studyUID"), r.PathValue("seriesUID"), r.URL.Query())
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	if len(objects) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSONMetadata(w, objects)
}

// ── response helpers ──────────────────────────────────────────────────────────

// writeMultipartDICOM writes objects as a multipart/related WADO-RS response.
func writeMultipartDICOM(w http.ResponseWriter, objects []media.DICOMObject) {
	mw := multipart.NewWriter(w)
	w.Header().Set("Content-Type", fmt.Sprintf(
		`multipart/related; type="application/dicom"; boundary=%s`, mw.Boundary()))
	for _, obj := range objects {
		part, partErr := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type": []string{"application/dicom"},
		})
		if partErr != nil {
			break
		}
		// Stream straight into the multipart part: buffering the object first
		// would hold a second full copy of its pixel data in memory.
		if _, writeErr := obj.WriteTo(part); writeErr != nil {
			if err := media.ValidateFileWrite(obj); err != nil {
				slog.Warn("wado: DICOM object not serializable", "err", err)
			} else {
				slog.Warn("wado: failed to write DICOM object", "err", writeErr)
			}
			break
		}
	}
	_ = mw.Close()
}

// dicomJSONTag is a single tag entry in DICOMweb JSON format.
type dicomJSONTag struct {
	VR    string        `json:"vr"`
	Value []interface{} `json:"Value,omitempty"`
}

// writeJSONMetadata serializes objects to DICOMweb JSON (application/dicom+json),
// excluding pixel data.
func writeJSONMetadata(w http.ResponseWriter, objects []media.DICOMObject) {
	result := make([]map[string]dicomJSONTag, 0, len(objects))
	for _, obj := range objects {
		result = append(result, objectToJSONTags(obj))
	}
	w.Header().Set("Content-Type", "application/dicom+json")
	_ = json.NewEncoder(w).Encode(result)
}

// objectToJSONTags converts a DICOMObject to the DICOMweb JSON tag map.
// Pixel data (7FE0,0010) is omitted.
func objectToJSONTags(obj media.DICOMObject) map[string]dicomJSONTag {
	result := make(map[string]dicomJSONTag)
	for _, tag := range obj.GetTags() {
		// Omit pixel data from JSON representations.
		if tag.Group == 0x7FE0 && tag.Element == 0x0010 {
			continue
		}
		key := fmt.Sprintf("%04X%04X", tag.Group, tag.Element)
		entry := dicomJSONTag{VR: tag.VR}
		switch tag.VR {
		case "US":
			entry.Value = []interface{}{tag.GetUint16()}
		case "UL":
			entry.Value = []interface{}{tag.GetUint32()}
		case "FL":
			entry.Value = []interface{}{tag.GetFloat()}
		case "FD":
			entry.Value = []interface{}{tag.GetFloat64()}
		case "OB", "OW", "OD", "OF", "UN":
			// Inline small bulk data; larger values are omitted for JSON responses.
			if len(tag.Data) > 0 && len(tag.Data) <= 4096 {
				entry.Value = []interface{}{tag.Data}
			}
		default:
			if s := tag.GetString(); s != "" {
				entry.Value = []interface{}{s}
			}
		}
		result[key] = entry
	}
	return result
}

// buildStowResponse constructs the STOW-RS JSON success response body.
func buildStowResponse(objects []media.DICOMObject) map[string]interface{} {
	items := make([]interface{}, 0, len(objects))
	for _, obj := range objects {
		items = append(items, map[string]interface{}{
			"00081150": map[string]interface{}{"vr": "UI", "Value": []string{obj.GetString(tags.SOPClassUID)}},
			"00081155": map[string]interface{}{"vr": "UI", "Value": []string{obj.GetString(tags.SOPInstanceUID)}},
		})
	}
	return map[string]interface{}{
		"00081199": map[string]interface{}{"vr": "SQ", "Value": items},
	}
}

// parseFrameList parses a comma-separated list of 1-based frame numbers.
func parseFrameList(s string) ([]int, error) {
	parts := strings.Split(s, ",")
	frames := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("invalid frame number %q", p)
		}
		frames = append(frames, n)
	}
	return frames, nil
}

func httpError(w http.ResponseWriter, err error, code int) {
	slog.Error("WADO error", "err", err)
	http.Error(w, err.Error(), code)
}

// ── WADO-RS delete handlers ───────────────────────────────────────────────────

func (s *wadoServer) deleteStudy(w http.ResponseWriter, r *http.Request) {
	if !requireUIDs(w, r.PathValue("studyUID")) {
		return
	}
	if err := s.params.Store.DeleteStudy(r.Context(), r.PathValue("studyUID")); err != nil {
		httpError(w, err, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *wadoServer) deleteSeries(w http.ResponseWriter, r *http.Request) {
	if !requireUIDs(w, r.PathValue("studyUID"), r.PathValue("seriesUID")) {
		return
	}
	if err := s.params.Store.DeleteSeries(r.Context(),
		r.PathValue("studyUID"), r.PathValue("seriesUID")); err != nil {
		httpError(w, err, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *wadoServer) deleteInstance(w http.ResponseWriter, r *http.Request) {
	if !requireUIDs(w, r.PathValue("studyUID"), r.PathValue("seriesUID"), r.PathValue("sopInstanceUID")) {
		return
	}
	if err := s.params.Store.DeleteInstance(r.Context(),
		r.PathValue("studyUID"), r.PathValue("seriesUID"), r.PathValue("sopInstanceUID")); err != nil {
		httpError(w, err, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// limitedReadAll reads at most 64 MiB from r to prevent memory exhaustion.
const maxPartBytes = 64 * 1024 * 1024

// maxStoreBodyBytes and maxStoreParts bound a STOW-RS upload as a whole.
// maxPartBytes caps each part, but nothing capped how many parts arrived or the
// total body, and every parsed object was accumulated before any was handed to
// the Store: a 1 GB body of 2000 parts held ~1.6 GB resident, with no ceiling
// other than the client's patience.
const (
	maxStoreBodyBytes = 2 << 30 // 2 GiB
	maxStoreParts     = 10000
)

// limitedReadAll reads at most maxPartBytes and reports whether the limit was
// hit, rather than silently returning a truncated part. A truncated DICOM
// payload fails to parse, so previously an oversized part vanished into the
// "skip and keep going" path and the response still said 200.
func limitedReadAll(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxPartBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxPartBytes {
		return nil, fmt.Errorf("part exceeds %d bytes", maxPartBytes)
	}
	return data, nil
}

// uidPattern is the DICOM UI value representation: dot-separated numeric
// components, at most 64 characters (PS3.5 §9.1).
var uidPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+)*$`)

// validUID reports whether s is a syntactically valid DICOM UID.
//
// Go's ServeMux percent-decodes path wildcards, so a request for
// `..%2f..%2f..%2fetc%2fpasswd` reaches the handler as the literal string
// `../../../etc/passwd`, and `a%00b` arrives with an embedded NUL. Those went
// to the Store verbatim. io-scp's Store resolves UIDs through a query engine so
// it is unaffected, but the Store interface documents no validation obligation,
// and the canonical DICOM layout is {studyUID}/{seriesUID}/{sopUID}.dcm — any
// filesystem- or object-store-backed implementation would inherit an arbitrary
// read/delete primitive.
func validUID(s string) bool {
	return s != "" && len(s) <= 64 && uidPattern.MatchString(s)
}

// requireUIDs validates every supplied path UID, writing 400 and returning
// false on the first invalid one.
func requireUIDs(w http.ResponseWriter, uids ...string) bool {
	for _, uid := range uids {
		if !validUID(uid) {
			http.Error(w, "invalid DICOM UID in request path", http.StatusBadRequest)
			return false
		}
	}
	return true
}
