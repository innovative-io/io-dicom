package main

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/innovative-io/io-dicom/clients/httpclient"
)

const dictionaryURL string = "https://raw.githubusercontent.com/fo-dicom/fo-dicom/development/FO-DICOM.Core/Dictionaries/DICOM%20Dictionary.xml"

type xmlDictionary struct {
	XMLName xml.Name           `xml:"dictionary"`
	Tags    []xmlDictionaryTag `xml:"tag"`
	UIDs    []xmlDictionaryUID `xml:"uid"`
}

type xmlDictionaryTag struct {
	Group   string `xml:"group,attr"`
	Element string `xml:"element,attr"`
	Keyword string `xml:"keyword,attr"`
	VR      string `xml:"vr,attr"`
	VM      string `xml:"vm,attr"`
	Name    string `xml:",chardata"`
}

type xmlDictionaryUID struct {
	UID     string `xml:"uid,attr"`
	Keyword string `xml:"keyword,attr"`
	Type    string `xml:"type,attr"`
	Name    string `xml:",chardata"`
}

func main() {
	root := findModuleRoot()
	tags, uids := downloadDictionary()
	writeCodingSchemesFile(filepath.Join(root, "dictionary/codingscheme/coding_schemes.go"), uids)
	writeDicomTags(filepath.Join(root, "dictionary/tags/dicom_tags.go"), tags)
	writeSOPClassesFile(filepath.Join(root, "dictionary/sopclass/sop_classes.go"), uids)
	writeTransferSyntaxesFile(filepath.Join(root, "dictionary/transfersyntax/transfer_syntaxes.go"), uids)
}

// findModuleRoot walks up from the current directory to find the go.mod file.
func findModuleRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			log.Fatal("could not find module root (no go.mod found)")
		}
		dir = parent
	}
}

func downloadDictionary() ([]xmlDictionaryTag, []xmlDictionaryUID) {
	params := httpclient.HTTPClientParams{
		URL: dictionaryURL,
	}
	client := httpclient.NewHTTPClient(params)
	response, err := client.Get()
	if err != nil {
		log.Fatal(err)
	}

	dict := new(xmlDictionary)
	err = xml.Unmarshal(response, dict)
	if err != nil {
		log.Fatal(err)
	}
	return dict.Tags, dict.UIDs
}

func writeCodingSchemesFile(path string, uids []xmlDictionaryUID) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	codingSchemes := make([]string, 0)

	fmt.Fprintln(w, "package codingscheme")
	fmt.Fprintln(w)

	for _, uid := range uids {
		if uid.Type != "Coding Scheme" {
			continue
		}
		codingSchemes = append(codingSchemes, uid.Keyword)
		fmt.Fprintf(w, "// %s - (%s) %s\n", uid.Keyword, uid.UID, uid.Name)
		fmt.Fprintf(w, "var %s = &CodingScheme{\n", uid.Keyword)
		fmt.Fprintf(w, "  UID: \"%s\",\n", uid.UID)
		fmt.Fprintf(w, "  Name: \"%s\",\n", uid.Keyword)
		uid.Name = strings.ReplaceAll(uid.Name, " (Retired)", "")
		fmt.Fprintf(w, "  Description: \"%s\",\n", uid.Name)
		fmt.Fprintf(w, "  Type: \"%s\",\n", uid.Type)
		fmt.Fprintln(w, "}")
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "var codingSchemes = []*CodingScheme{")
	for _, cs := range codingSchemes {
		fmt.Fprintf(w, "  %s,\n", cs)
	}
	fmt.Fprintln(w, "}")

	if err := w.Flush(); err != nil {
		log.Fatal(err)
	}
}

func writeDicomTags(path string, tags []xmlDictionaryTag) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	dicomTags := make([]string, 0)

	fmt.Fprintln(w, "package tags")
	fmt.Fprintln(w)

	for _, tag := range tags {
		if strings.Contains(tag.Group, "x") || strings.Contains(tag.Element, "x") {
			continue
		}
		dicomTags = append(dicomTags, tag.Keyword)
		fmt.Fprintf(w, "// %s - (%s,%s) %s\n", tag.Keyword, tag.Group, tag.Element, tag.Name)
		fmt.Fprintf(w, "var %s = &Tag{\n", tag.Keyword)
		fmt.Fprintf(w, "  Group: 0x%s,\n", tag.Group)
		fmt.Fprintf(w, "  Element: 0x%s,\n", tag.Element)
		fmt.Fprintf(w, "  VR: \"%s\",\n", tag.VR)
		fmt.Fprintf(w, "  VM: \"%s\",\n", tag.VM)
		fmt.Fprintf(w, "  Name: \"%s\",\n", tag.Keyword)
		tag.Name = strings.ReplaceAll(tag.Name, " (Trial)", "")
		tag.Name = strings.ReplaceAll(tag.Name, " (Retired)", "")
		fmt.Fprintf(w, "  Description: \"%s\",\n", tag.Name)
		fmt.Fprintln(w, "}")
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "var tags = []*Tag{")
	for _, tag := range dicomTags {
		fmt.Fprintf(w, "  %s,\n", tag)
	}
	fmt.Fprintln(w, "}")

	if err := w.Flush(); err != nil {
		log.Fatal(err)
	}
}

func writeSOPClassesFile(path string, uids []xmlDictionaryUID) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	sopClasses := make([]string, 0)

	fmt.Fprintln(w, "package sopclass")
	fmt.Fprintln(w)

	for _, uid := range uids {
		if uid.Type != "SOP Class" && uid.Type != "Application Context Name" {
			continue
		}
		sopClasses = append(sopClasses, uid.Keyword)
		fmt.Fprintf(w, "// %s - (%s) %s\n", uid.Keyword, uid.UID, uid.Name)
		fmt.Fprintf(w, "var %s = &SOPClass{\n", uid.Keyword)
		fmt.Fprintf(w, "  UID: \"%s\",\n", uid.UID)
		fmt.Fprintf(w, "  Name: \"%s\",\n", uid.Keyword)
		uid.Name = strings.ReplaceAll(uid.Name, " (Retired)", "")
		fmt.Fprintf(w, "  Description: \"%s\",\n", uid.Name)
		fmt.Fprintf(w, "  Type: \"%s\",\n", uid.Type)
		fmt.Fprintln(w, "}")
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "var sopClasses = []*SOPClass{")
	for _, sopClass := range sopClasses {
		fmt.Fprintf(w, "  %s,\n", sopClass)
	}
	fmt.Fprintln(w, "}")

	if err := w.Flush(); err != nil {
		log.Fatal(err)
	}
}

func writeTransferSyntaxesFile(path string, uids []xmlDictionaryUID) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	transferSyntaxes := make([]string, 0)

	fmt.Fprintln(w, "package transfersyntax")
	fmt.Fprintln(w)

	for _, uid := range uids {
		if uid.Type != "Transfer Syntax" {
			continue
		}
		transferSyntaxes = append(transferSyntaxes, uid.Keyword)
		fmt.Fprintf(w, "// %s - (%s) %s\n", uid.Keyword, uid.UID, uid.Name)
		fmt.Fprintf(w, "var %s = &TransferSyntax{\n", uid.Keyword)
		fmt.Fprintf(w, "  UID: \"%s\",\n", uid.UID)
		fmt.Fprintf(w, "  Name: \"%s\",\n", uid.Keyword)
		uid.Name = strings.ReplaceAll(uid.Name, " (Retired)", "")
		description := strings.Split(uid.Name, ":")
		fmt.Fprintf(w, "  Description: \"%s\",\n", description[0])
		fmt.Fprintf(w, "  Type: \"%s\",\n", uid.Type)
		fmt.Fprintln(w, "}")
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "var transferSyntaxes = []*TransferSyntax{")
	for _, ts := range transferSyntaxes {
		fmt.Fprintf(w, "  %s,\n", ts)
	}
	fmt.Fprintln(w, "}")

	if err := w.Flush(); err != nil {
		log.Fatal(err)
	}
}
