package network

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"testing"

	"github.com/innovative-io/io-dicom/network/internal/pdutype"
)

func readSingleRawPDU(r io.Reader) ([]byte, error) {
	header := make([]byte, 10)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	pduLength := int(header[2])<<24 | int(header[3])<<16 | int(header[4])<<8 | int(header[5])
	if pduLength < 4 {
		return nil, io.ErrUnexpectedEOF
	}
	remaining := pduLength - 4
	data := append([]byte(nil), header...)
	if remaining == 0 {
		return data, nil
	}
	payload := make([]byte, remaining)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	return append(data, payload...), nil
}

func encodedPDUBytes(t *testing.T, encode func(rw *bufio.ReadWriter) error) []byte {
	t.Helper()
	var buffer bytes.Buffer
	rw := bufio.NewReadWriter(bufio.NewReader(bytes.NewReader(nil)), bufio.NewWriter(&buffer))
	if err := encode(rw); err != nil {
		t.Fatalf("encode PDU: %v", err)
	}
	if err := rw.Flush(); err != nil {
		t.Fatalf("flush encoded PDU: %v", err)
	}
	return append([]byte(nil), buffer.Bytes()...)
}

func TestCloseEmitsRawPDUEventsForReleaseHandshake(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	pdu := NewPDUService().(*pduService)
	pdu.SetConn(bufio.NewReadWriter(bufio.NewReader(clientConn), bufio.NewWriter(clientConn)))
	pdu.SetNetConn(clientConn)

	var events []RawPDUEvent
	pdu.SetOnRawPDU(func(event RawPDUEvent) {
		events = append(events, event)
	})

	expectedInbound := encodedPDUBytes(t, func(rw *bufio.ReadWriter) error {
		return NewReleaseResponse().Write(rw)
	})
	serverRequest := make(chan []byte, 1)
	serverDone := make(chan error, 1)
	go func() {
		rw := bufio.NewReadWriter(bufio.NewReader(serverConn), bufio.NewWriter(serverConn))
		requestBytes, err := readSingleRawPDU(rw)
		if err != nil {
			serverDone <- err
			return
		}
		serverRequest <- requestBytes
		if _, err := rw.Write(expectedInbound); err != nil {
			serverDone <- err
			return
		}
		serverDone <- rw.Flush()
	}()

	pdu.Close()

	if err := <-serverDone; err != nil {
		t.Fatalf("server handshake failed: %v", err)
	}
	requestBytes := <-serverRequest
	if len(events) != 2 {
		t.Fatalf("expected 2 raw PDU events, got %d", len(events))
	}
	if events[0].Direction != RawPDUDirectionOutbound || events[0].PDUType != byte(pdutype.AssociationReleaseRequest) {
		t.Fatalf("unexpected first event: %+v", events[0])
	}
	if events[1].Direction != RawPDUDirectionInbound || events[1].PDUType != byte(pdutype.AssociationReleaseResponse) {
		t.Fatalf("unexpected second event: %+v", events[1])
	}
	if len(events[0].Data) == 0 || len(events[1].Data) == 0 {
		t.Fatal("expected captured raw PDU bytes for both handshake events")
	}
	if !bytes.Equal(events[0].Data, requestBytes) {
		t.Fatalf("outbound raw bytes do not match bytes observed by peer\ncallback=% X\npeer=% X", events[0].Data, requestBytes)
	}
	if !bytes.Equal(events[1].Data, expectedInbound) {
		t.Fatalf("inbound raw bytes do not match bytes sent by peer\ncallback=% X\npeer=% X", events[1].Data, expectedInbound)
	}
}
