// index-dicom scans a directory of DICOM files and extracts patient/study/series/
// instance metadata using a fast streaming parser that never loads pixel data.
//
// Usage:
//
//	index-dicom --dir /path/to/dicoms [--workers N] [--compare] [--verbose] [--limit N]
//
// Flags:
//
//	--dir      Directory to scan recursively (required).
//	--workers  Worker goroutines (default: number of CPU cores).
//	--compare  Run both old full-parse and new index-parse and print a timing comparison.
//	--verbose  Print the extracted IndexRecord for each successfully parsed file.
//	--limit    Process at most N files (0 = no limit; useful for quick tests).
package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/innovative-io/io-dicom/media"
)

func main() {
	dir := flag.String("dir", "", "directory to scan for DICOM files (required)")
	workers := flag.Int("workers", runtime.NumCPU(), "number of worker goroutines")
	compare := flag.Bool("compare", false, "compare old full-parse vs new index-parse timing")
	verbose := flag.Bool("verbose", false, "print extracted record for each file")
	limit := flag.Int("limit", 0, "process at most N files (0 = no limit)")
	flag.Parse()

	if *dir == "" {
		flag.Usage()
		os.Exit(1)
	}

	paths := collectFiles(*dir)
	if len(paths) == 0 {
		log.Fatalf("no files found under %s", *dir)
	}
	if *limit > 0 && len(paths) > *limit {
		paths = paths[:*limit]
	}

	fmt.Printf("Found %d files under %s\n", len(paths), *dir)
	fmt.Printf("Workers: %d\n\n", *workers)

	if *compare {
		runComparison(paths, *workers, *verbose)
	} else {
		runIndex(paths, *workers, *verbose)
	}
}

// collectFiles walks dir recursively and returns all regular file paths.
// Every regular file is a candidate — non-DICOM files are simply skipped
// by ParseIndexFromFile when the DICM preamble is absent.
func collectFiles(dir string) []string {
	var paths []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries, continue walk
		}
		if !d.IsDir() {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		log.Printf("walk warning: %v", err)
	}
	return paths
}

type result struct {
	parsed   int64
	notDICOM int64
	failed   int64
}

// runIndex indexes all files using ParseIndexFromFile and reports throughput.
func runIndex(paths []string, nWorkers int, verbose bool) {
	pathCh := make(chan string, nWorkers*4)
	var res result
	var wg sync.WaitGroup

	start := time.Now()

	for range nWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range pathCh {
				rec, err := media.ParseIndexFromFile(path)
				if err != nil {
					if isNotDICOM(err) {
						atomic.AddInt64(&res.notDICOM, 1)
					} else {
						atomic.AddInt64(&res.failed, 1)
						log.Printf("error: %s: %v", path, err)
					}
					continue
				}
				atomic.AddInt64(&res.parsed, 1)
				if verbose {
					printRecord(rec)
				}
			}
		}()
	}

	for _, p := range paths {
		pathCh <- p
	}
	close(pathCh)
	wg.Wait()

	elapsed := time.Since(start)
	fmt.Printf("[index-parse]\n")
	fmt.Printf("  Parsed:    %d\n", res.parsed)
	fmt.Printf("  Not DICOM: %d\n", res.notDICOM)
	fmt.Printf("  Errors:    %d\n", res.failed)
	fmt.Printf("  Time:      %v\n", elapsed.Round(time.Millisecond))
	if elapsed > 0 && res.parsed > 0 {
		fmt.Printf("  Rate:      %.1f files/sec\n", float64(res.parsed)/elapsed.Seconds())
	}
}

// runComparison runs two passes over the same file set — first using
// NewDCMObjFromFile (full parse, reads every byte), then ParseIndexFromFile
// (metadata-only, capped at 128 KiB per file) — and prints a side-by-side
// timing comparison.
//
// Both passes run with the same worker count. The second pass benefits from
// the OS page cache warmed by the first, so any speedup measured here is a
// conservative lower bound on cold-cache gains (which are even larger for
// multi-megabyte files).
func runComparison(paths []string, nWorkers int, verbose bool) {
	fmt.Println("Pass 1/2 — BEFORE: full parse (NewDCMObjFromFile, reads entire file)")
	res1, elapsed1 := runFullParse(paths, nWorkers)

	fmt.Printf("  Parsed: %d  Not-DICOM: %d  Errors: %d  Time: %v  Rate: %.1f files/sec\n\n",
		res1.parsed, res1.notDICOM, res1.failed,
		elapsed1.Round(time.Millisecond),
		rate(res1.parsed, elapsed1))

	fmt.Println("Pass 2/2 — AFTER:  index parse (ParseIndexFromFile, reads ≤128 KiB)")
	res2, elapsed2 := runIndexParse(paths, nWorkers, verbose)

	fmt.Printf("  Parsed: %d  Not-DICOM: %d  Errors: %d  Time: %v  Rate: %.1f files/sec\n\n",
		res2.parsed, res2.notDICOM, res2.failed,
		elapsed2.Round(time.Millisecond),
		rate(res2.parsed, elapsed2))

	if elapsed2 > 0 && elapsed1 > 0 {
		speedup := float64(elapsed1) / float64(elapsed2)
		fmt.Printf("Speedup: %.2fx faster  (%.1f → %.1f files/sec)\n",
			speedup, rate(res1.parsed, elapsed1), rate(res2.parsed, elapsed2))
	}
}

func runFullParse(paths []string, nWorkers int) (result, time.Duration) {
	pathCh := make(chan string, nWorkers*4)
	var res result
	var wg sync.WaitGroup

	start := time.Now()

	for range nWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range pathCh {
				obj, err := media.NewDCMObjFromFile(path)
				if err != nil {
					if isNotDICOM(err) {
						atomic.AddInt64(&res.notDICOM, 1)
					} else {
						atomic.AddInt64(&res.failed, 1)
					}
					continue
				}
				if obj == nil {
					atomic.AddInt64(&res.failed, 1)
					continue
				}
				atomic.AddInt64(&res.parsed, 1)
			}
		}()
	}

	for _, p := range paths {
		pathCh <- p
	}
	close(pathCh)
	wg.Wait()
	return res, time.Since(start)
}

func runIndexParse(paths []string, nWorkers int, verbose bool) (result, time.Duration) {
	pathCh := make(chan string, nWorkers*4)
	var res result
	var wg sync.WaitGroup

	start := time.Now()

	for range nWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range pathCh {
				rec, err := media.ParseIndexFromFile(path)
				if err != nil {
					if isNotDICOM(err) {
						atomic.AddInt64(&res.notDICOM, 1)
					} else {
						atomic.AddInt64(&res.failed, 1)
					}
					continue
				}
				atomic.AddInt64(&res.parsed, 1)
				if verbose {
					printRecord(rec)
				}
			}
		}()
	}

	for _, p := range paths {
		pathCh <- p
	}
	close(pathCh)
	wg.Wait()
	return res, time.Since(start)
}

func isNotDICOM(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, media.ErrNotDICOM) ||
		errors.Is(err, media.ErrFileTooSmall) ||
		errors.Is(err, media.ErrNoTransferSyntax)
}

func rate(n int64, d time.Duration) float64 {
	if d <= 0 || n == 0 {
		return 0
	}
	return float64(n) / d.Seconds()
}

func printRecord(rec *media.IndexRecord) {
	fmt.Printf("%-50s  %-10s  %-2s  %s\n",
		rec.FilePath,
		rec.PatientID,
		rec.Modality,
		rec.StudyInstanceUID)
}
