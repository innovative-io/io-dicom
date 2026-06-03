package media

import (
	"io"
	"log"
	"os"
	"sync"
	"testing"

	"github.com/innovative-io/io-dicom/dictionary/tags"
)

var benchmarkLogOnce sync.Once

func muteParserLogsForBenchmarks() {
	benchmarkLogOnce.Do(func() {
		log.SetOutput(io.Discard)
	})
}

func benchmarkNewDCMObjFromFile(b *testing.B, fileName string) {
	b.Helper()
	muteParserLogsForBenchmarks()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		obj, err := NewDCMObjFromFile(fileName)
		if err != nil {
			b.Fatalf("NewDCMObjFromFile(%s) failed: %v", fileName, err)
		}
		if obj == nil {
			b.Fatalf("NewDCMObjFromFile(%s) returned nil object", fileName)
		}
	}
}

func benchmarkNewDCMObjFromBytes(b *testing.B, fileName string) {
	b.Helper()
	muteParserLogsForBenchmarks()
	data, err := os.ReadFile(fileName)
	if err != nil {
		b.Fatalf("failed to read sample %s: %v", fileName, err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		obj, err := NewDCMObjFromBytes(data)
		if err != nil {
			b.Fatalf("NewDCMObjFromBytes(%s) failed: %v", fileName, err)
		}
		if obj == nil {
			b.Fatalf("NewDCMObjFromBytes(%s) returned nil object", fileName)
		}
	}
}

func benchmarkWriteToBytes(b *testing.B, fileName string) {
	b.Helper()
	muteParserLogsForBenchmarks()

	obj, err := NewDCMObjFromFile(fileName)
	if err != nil {
		b.Fatalf("failed to load sample %s: %v", fileName, err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		data := obj.WriteToBytes()
		if len(data) == 0 {
			b.Fatalf("WriteToBytes(%s) returned empty payload", fileName)
		}
	}
}

func BenchmarkNewDCMObjFromFile_Test2(b *testing.B) {
	benchmarkNewDCMObjFromFile(b, "../testdata/test2.dcm")
}

func BenchmarkNewDCMObjFromBytes_Test2(b *testing.B) {
	benchmarkNewDCMObjFromBytes(b, "../testdata/test2.dcm")
}

func BenchmarkNewDCMObjFromFile_JPEG8(b *testing.B) {
	benchmarkNewDCMObjFromFile(b, "../testdata/jpeg8.dcm")
}

func BenchmarkNewDCMObjFromBytes_JPEG8(b *testing.B) {
	benchmarkNewDCMObjFromBytes(b, "../testdata/jpeg8.dcm")
}

func BenchmarkWriteToBytes_Test2(b *testing.B) {
	benchmarkWriteToBytes(b, "../testdata/test2.dcm")
}

func BenchmarkWriteToBytes_JPEG8(b *testing.B) {
	benchmarkWriteToBytes(b, "../testdata/jpeg8.dcm")
}

// --- ParseIndex benchmarks (fast index path) ---

func benchmarkParseIndexFromFile(b *testing.B, fileName string) {
	b.Helper()
	muteParserLogsForBenchmarks()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rec, err := ParseIndexFromFile(fileName)
		if err != nil {
			b.Fatalf("ParseIndexFromFile(%s) failed: %v", fileName, err)
		}
		if rec == nil {
			b.Fatalf("ParseIndexFromFile(%s) returned nil record", fileName)
		}
	}
}

func benchmarkParseIndexFromBytes(b *testing.B, fileName string) {
	b.Helper()
	muteParserLogsForBenchmarks()
	data, err := os.ReadFile(fileName)
	if err != nil {
		b.Fatalf("failed to read sample %s: %v", fileName, err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rec, err := ParseIndexFromBytes(data)
		if err != nil {
			b.Fatalf("ParseIndexFromBytes(%s) failed: %v", fileName, err)
		}
		if rec == nil {
			b.Fatalf("ParseIndexFromBytes(%s) returned nil record", fileName)
		}
	}
}

func BenchmarkParseIndexFromFile_Test2(b *testing.B) {
	benchmarkParseIndexFromFile(b, "../testdata/test2.dcm")
}

func BenchmarkParseIndexFromFile_JPEG8(b *testing.B) {
	benchmarkParseIndexFromFile(b, "../testdata/jpeg8.dcm")
}

func BenchmarkParseIndexFromBytes_Test2(b *testing.B) {
	benchmarkParseIndexFromBytes(b, "../testdata/test2.dcm")
}

func BenchmarkParseIndexFromBytes_JPEG8(b *testing.B) {
	benchmarkParseIndexFromBytes(b, "../testdata/jpeg8.dcm")
}

// --- GetTag benchmarks ---

func benchmarkGetTag(b *testing.B, fileName string) {
	b.Helper()
	muteParserLogsForBenchmarks()
	obj, err := NewDCMObjFromFile(fileName)
	if err != nil {
		b.Fatalf("NewDCMObjFromFile(%s) failed: %v", fileName, err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = obj.GetTag(tags.PatientName)
		_ = obj.GetTag(tags.StudyInstanceUID)
		_ = obj.GetTag(tags.SOPInstanceUID)
	}
}

func BenchmarkGetTag_Test2(b *testing.B) { benchmarkGetTag(b, "../testdata/test2.dcm") }
func BenchmarkGetTag_JPEG8(b *testing.B) { benchmarkGetTag(b, "../testdata/jpeg8.dcm") }

// --- DumpTags benchmarks ---

func benchmarkDumpTags(b *testing.B, fileName string) {
	b.Helper()
	muteParserLogsForBenchmarks()
	obj, err := NewDCMObjFromFile(fileName)
	if err != nil {
		b.Fatalf("NewDCMObjFromFile(%s) failed: %v", fileName, err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		obj.DumpTags(io.Discard)
	}
}

func BenchmarkDumpTags_Test2(b *testing.B) { benchmarkDumpTags(b, "../testdata/test2.dcm") }
func BenchmarkDumpTags_JPEG8(b *testing.B) { benchmarkDumpTags(b, "../testdata/jpeg8.dcm") }
