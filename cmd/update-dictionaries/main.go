package main

import (
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	defaultDICOMDictURL = "https://raw.githubusercontent.com/pydicom/pydicom/main/src/pydicom/_dicom_dict.py"
	defaultUIDDictURL   = "https://raw.githubusercontent.com/pydicom/pydicom/main/src/pydicom/_uid_dict.py"
)

type tagEntry struct {
	Tag         uint32
	VR          string
	VM          string
	Description string
	Keyword     string
}

type uidEntry struct {
	UID     string
	Name    string
	Type    string
	Info    string
	Retired string
	Keyword string
}

func main() {
	repoRoot := flag.String("repo-root", defaultRepoRoot(), "Repository root containing dictionary/ and cmd/")
	dicomDictURL := flag.String("dicom-dict-url", defaultDICOMDictURL, "URL for pydicom DICOM dictionary source")
	uidDictURL := flag.String("uid-dict-url", defaultUIDDictURL, "URL for pydicom UID dictionary source")
	flag.Parse()

	if err := run(*repoRoot, *dicomDictURL, *uidDictURL); err != nil {
		fmt.Fprintf(os.Stderr, "update dictionaries: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Updated dictionary/tags/dicom_tags.go and dictionary/transfersyntax/transfer_syntaxes.go")
}

func run(repoRoot string, dicomDictURL string, uidDictURL string) error {
	dicomSource, err := fetch(dicomDictURL)
	if err != nil {
		return fmt.Errorf("fetch dicom dictionary: %w", err)
	}
	uidSource, err := fetch(uidDictURL)
	if err != nil {
		return fmt.Errorf("fetch uid dictionary: %w", err)
	}

	tagEntries, err := parseDicomDictionary(dicomSource)
	if err != nil {
		return fmt.Errorf("parse dicom dictionary: %w", err)
	}
	uidEntries, err := parseUIDDictionary(uidSource)
	if err != nil {
		return fmt.Errorf("parse uid dictionary: %w", err)
	}

	tagsFile, err := buildTagsFile(tagEntries)
	if err != nil {
		return fmt.Errorf("build tags file: %w", err)
	}
	transferSyntaxFile, err := buildTransferSyntaxFile(uidEntries)
	if err != nil {
		return fmt.Errorf("build transfer syntax file: %w", err)
	}

	if err := os.WriteFile(filepath.Join(repoRoot, "dictionary/tags/dicom_tags.go"), tagsFile, 0o644); err != nil {
		return fmt.Errorf("write dicom tags file: %w", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "dictionary/transfersyntax/transfer_syntaxes.go"), transferSyntaxFile, 0o644); err != nil {
		return fmt.Errorf("write transfer syntax file: %w", err)
	}

	return nil
}

func defaultRepoRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func fetch(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func parseDicomDictionary(src string) ([]tagEntry, error) {
	lines, err := extractDictionaryLines(src, "DicomDictionary")
	if err != nil {
		return nil, err
	}

	entries := make([]tagEntry, 0, len(lines))
	for _, line := range lines {
		colonIndex := strings.Index(line, ":")
		if colonIndex < 0 {
			return nil, fmt.Errorf("invalid dicom entry %q", line)
		}

		tagValue, err := strconv.ParseUint(strings.TrimSpace(line[:colonIndex]), 0, 32)
		if err != nil {
			return nil, fmt.Errorf("parse tag value from %q: %w", line, err)
		}

		values, err := parsePythonTuple(strings.TrimSuffix(strings.TrimSpace(line[colonIndex+1:]), ","))
		if err != nil {
			return nil, fmt.Errorf("parse tag tuple from %q: %w", line, err)
		}
		if len(values) != 5 {
			return nil, fmt.Errorf("expected 5 tag tuple values, got %d", len(values))
		}

		entries = append(entries, tagEntry{
			Tag:         uint32(tagValue),
			VR:          values[0],
			VM:          values[1],
			Description: values[2],
			Keyword:     values[4],
		})
	}

	slices.SortFunc(entries, func(left tagEntry, right tagEntry) int {
		return compareUint32(left.Tag, right.Tag)
	})

	return entries, nil
}

func parseUIDDictionary(src string) ([]uidEntry, error) {
	lines, err := extractDictionaryLines(src, "UID_dictionary")
	if err != nil {
		return nil, err
	}

	entries := make([]uidEntry, 0, len(lines))
	for _, line := range lines {
		key, remainder, err := parsePythonString(strings.TrimSpace(line), 0)
		if err != nil {
			return nil, fmt.Errorf("parse uid key from %q: %w", line, err)
		}

		remainder = strings.TrimSpace(remainder)
		if !strings.HasPrefix(remainder, ":") {
			return nil, fmt.Errorf("missing ':' in uid entry %q", line)
		}

		values, err := parsePythonTuple(strings.TrimSuffix(strings.TrimSpace(remainder[1:]), ","))
		if err != nil {
			return nil, fmt.Errorf("parse uid tuple from %q: %w", line, err)
		}
		if len(values) != 5 {
			return nil, fmt.Errorf("expected 5 uid tuple values, got %d", len(values))
		}

		entries = append(entries, uidEntry{
			UID:     key,
			Name:    values[0],
			Type:    values[1],
			Info:    values[2],
			Retired: values[3],
			Keyword: values[4],
		})
	}

	slices.SortFunc(entries, func(left uidEntry, right uidEntry) int {
		return compareUID(left.UID, right.UID)
	})

	return entries, nil
}

func extractDictionaryLines(src string, name string) ([]string, error) {
	lines := strings.Split(src, "\n")
	inDictionary := false
	entries := make([]string, 0)

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if !inDictionary {
			if strings.HasPrefix(line, name) && strings.Contains(line, "{") {
				inDictionary = true
			}
			continue
		}

		if line == "}" {
			return entries, nil
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entries = append(entries, line)
	}

	return nil, fmt.Errorf("dictionary %s not found", name)
}

func parsePythonTuple(src string) ([]string, error) {
	src = strings.TrimSpace(src)
	if len(src) < 2 || src[0] != '(' || src[len(src)-1] != ')' {
		return nil, fmt.Errorf("invalid tuple %q", src)
	}

	values := make([]string, 0, 5)
	remaining := src[1 : len(src)-1]
	for {
		remaining = strings.TrimSpace(remaining)
		if remaining == "" {
			return values, nil
		}

		value, tail, err := parsePythonString(remaining, 0)
		if err != nil {
			return nil, err
		}
		values = append(values, value)

		tail = strings.TrimSpace(tail)
		if tail == "" {
			return values, nil
		}
		if !strings.HasPrefix(tail, ",") {
			return nil, fmt.Errorf("invalid tuple separator in %q", src)
		}
		remaining = tail[1:]
	}
}

func parsePythonString(src string, start int) (string, string, error) {
	if start >= len(src) {
		return "", "", io.ErrUnexpectedEOF
	}

	quote := src[start]
	if quote != '\'' && quote != '"' {
		return "", "", fmt.Errorf("expected quoted string in %q", src)
	}

	var builder strings.Builder
	for index := start + 1; index < len(src); index++ {
		ch := src[index]
		switch ch {
		case quote:
			return builder.String(), src[index+1:], nil
		case '\\':
			decoded, width, err := decodePythonEscape(src[index:])
			if err != nil {
				return "", "", err
			}
			builder.WriteString(decoded)
			index += width - 1
		default:
			builder.WriteByte(ch)
		}
	}

	return "", "", io.ErrUnexpectedEOF
}

func decodePythonEscape(src string) (string, int, error) {
	if len(src) < 2 || src[0] != '\\' {
		return "", 0, errors.New("invalid escape sequence")
	}

	switch src[1] {
	case '\\', '\'', '"':
		return string(src[1]), 2, nil
	case 'n':
		return "\n", 2, nil
	case 'r':
		return "\r", 2, nil
	case 't':
		return "\t", 2, nil
	case 'x':
		if len(src) < 4 {
			return "", 0, io.ErrUnexpectedEOF
		}
		value, err := strconv.ParseUint(src[2:4], 16, 8)
		if err != nil {
			return "", 0, err
		}
		return string(rune(value)), 4, nil
	case 'u':
		if len(src) < 6 {
			return "", 0, io.ErrUnexpectedEOF
		}
		value, err := strconv.ParseUint(src[2:6], 16, 16)
		if err != nil {
			return "", 0, err
		}
		return string(rune(value)), 6, nil
	case 'U':
		if len(src) < 10 {
			return "", 0, io.ErrUnexpectedEOF
		}
		value, err := strconv.ParseUint(src[2:10], 16, 32)
		if err != nil {
			return "", 0, err
		}
		return string(rune(value)), 10, nil
	default:
		return string(src[1]), 2, nil
	}
}

func buildTagsFile(entries []tagEntry) ([]byte, error) {
	entries = slices.Clone(entries)
	slices.SortFunc(entries, func(left tagEntry, right tagEntry) int {
		return compareUint32(left.Tag, right.Tag)
	})

	var builder strings.Builder
	builder.WriteString("package tags\n\n")

	usedNames := make(map[string]struct{}, len(entries))
	generatedNames := make([]string, 0, len(entries))

	for _, entry := range entries {
		group := uint16(entry.Tag >> 16)
		element := uint16(entry.Tag)
		identifierBase := entry.Keyword
		if identifierBase == "" {
			identifierBase = entry.Description
		}
		identifier := sanitizeIdentifier(identifierBase)
		if _, exists := usedNames[identifier]; exists {
			identifier = fmt.Sprintf("%s%04X%04X", identifier, group, element)
		}
		usedNames[identifier] = struct{}{}
		generatedNames = append(generatedNames, identifier)

		fmt.Fprintf(&builder, "// %s - (%04X,%04X) %s\n", identifier, group, element, entry.Description)
		fmt.Fprintf(&builder, "var %s = &Tag{\n", identifier)
		fmt.Fprintf(&builder, "\tGroup:\t\t0x%04X,\n", group)
		fmt.Fprintf(&builder, "\tElement:\t0x%04X,\n", element)
		fmt.Fprintf(&builder, "\tVR:\t\t%q,\n", entry.VR)
		fmt.Fprintf(&builder, "\tVM:\t\t%q,\n", entry.VM)
		fmt.Fprintf(&builder, "\tName:\t\t%q,\n", identifier)
		fmt.Fprintf(&builder, "\tDescription:\t%q,\n", entry.Description)
		builder.WriteString("}\n\n")
	}

	builder.WriteString("var tags = []*Tag{\n")
	for _, name := range generatedNames {
		fmt.Fprintf(&builder, "\t%s,\n", name)
	}
	builder.WriteString("}\n")

	return format.Source([]byte(builder.String()))
}

func buildTransferSyntaxFile(entries []uidEntry) ([]byte, error) {
	entries = slices.Clone(entries)
	slices.SortFunc(entries, func(left uidEntry, right uidEntry) int {
		return compareUID(left.UID, right.UID)
	})

	var builder strings.Builder
	builder.WriteString("package transfersyntax\n\n")

	usedNames := make(map[string]struct{})
	generatedNames := make([]string, 0)

	for _, entry := range entries {
		if entry.Type != "Transfer Syntax" {
			continue
		}

		identifierBase := entry.Keyword
		if identifierBase == "" {
			identifierBase = entry.Name
		}
		identifier := sanitizeIdentifier(identifierBase)
		if _, exists := usedNames[identifier]; exists {
			identifier = fmt.Sprintf("%s%s", identifier, sanitizeIdentifier(entry.UID))
		}
		usedNames[identifier] = struct{}{}
		generatedNames = append(generatedNames, identifier)

		fmt.Fprintf(&builder, "// %s - (%s) %s\n", identifier, entry.UID, entry.Name)
		fmt.Fprintf(&builder, "var %s = &TransferSyntax{\n", identifier)
		fmt.Fprintf(&builder, "\tUID:\t\t%q,\n", entry.UID)
		fmt.Fprintf(&builder, "\tName:\t\t%q,\n", identifier)
		fmt.Fprintf(&builder, "\tDescription:\t%q,\n", entry.Name)
		builder.WriteString("\tType:\t\t\"Transfer Syntax\",\n")
		builder.WriteString("}\n\n")
	}

	builder.WriteString("var transferSyntaxes = []*TransferSyntax{\n")
	for _, name := range generatedNames {
		fmt.Fprintf(&builder, "\t%s,\n", name)
	}
	builder.WriteString("}\n")

	return format.Source([]byte(builder.String()))
}

func sanitizeIdentifier(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}

	identifier := builder.String()
	if identifier == "" {
		identifier = "Unknown"
	}
	first, _ := utf8.DecodeRuneInString(identifier)
	if unicode.IsDigit(first) {
		identifier = "Tag" + identifier
	}
	return identifier
}

func compareUint32(left uint32, right uint32) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareUID(left string, right string) int {
	leftParts := strings.Split(left, ".")
	rightParts := strings.Split(right, ".")
	limit := len(leftParts)
	if len(rightParts) < limit {
		limit = len(rightParts)
	}

	for index := 0; index < limit; index++ {
		leftValue, leftErr := strconv.Atoi(leftParts[index])
		rightValue, rightErr := strconv.Atoi(rightParts[index])
		switch {
		case leftErr == nil && rightErr == nil:
			if leftValue != rightValue {
				if leftValue < rightValue {
					return -1
				}
				return 1
			}
		default:
			if leftParts[index] != rightParts[index] {
				if leftParts[index] < rightParts[index] {
					return -1
				}
				return 1
			}
		}
	}

	switch {
	case len(leftParts) < len(rightParts):
		return -1
	case len(leftParts) > len(rightParts):
		return 1
	default:
		return 0
	}
}
