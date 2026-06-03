package uuids

import (
	"strings"
	"testing"
)

func TestCreateSeriesUID_Deterministic(t *testing.T) {
	a := CreateSeriesUID("1.2.3", "MR", "1")
	b := CreateSeriesUID("1.2.3", "MR", "1")
	if a != b {
		t.Fatalf("CreateSeriesUID() not deterministic: %q != %q", a, b)
	}
}

func TestCreateSeriesUID_DifferentModality(t *testing.T) {
	a := CreateSeriesUID("1.2.3", "CT", "1")
	b := CreateSeriesUID("1.2.3", "MR", "1")
	if a == b {
		t.Fatal("CreateSeriesUID() should differ for different modalities")
	}
}

func TestCreateSeriesUID_HasPrefix(t *testing.T) {
	rootUID := "1.2.3.4"
	uid := CreateSeriesUID(rootUID, "CT", "1")
	if !strings.HasPrefix(uid, rootUID+".") {
		t.Fatalf("CreateSeriesUID() = %q, should start with %q", uid, rootUID+".")
	}
}

func TestCreateInstanceUID_MultiDigit(t *testing.T) {
	roots := "1.2.826.0.1.3680043.10.90.999"
	uid := CreateInstanceUID(roots, "42")
	if !strings.HasSuffix(uid, ".42") {
		t.Fatalf("CreateInstanceUID() = %q, should end with .42", uid)
	}
}
