package main

import (
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/innovative-io/io-dicom/clients/httpclient"
)

const dictionaryURL string = "https://raw.githubusercontent.com/fo-dicom/fo-dicom/development/FO-DICOM.Core/Dictionaries/DICOM%20Dictionary.xml"

const codingSchemesFile string = "../../dictionary/codingscheme/coding_schemes.go"

const dicomTagsFile string = "../../dictionary/tags/dicom_tags.go"

const sopClassesFile string = "../../dictionary/sopclass/sop_classes.go"

const transferSyntaxesFile string = "../../dictionary/transfersyntax/transfer_syntaxes.go"

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
	tags, uids := downloadDictionary()
	writeCopdingSchemesFile(uids)
	writeDicomTags(tags)
	writeSOPClassesFile(uids)
	writeTransferSyntaxesFile(uids)
}

func downloadDictionary() ([]xmlDictionaryTag, []xmlDictionaryUID) {
	params := httpclient.HTTPClientParams{
		URL: dictionaryURL,
	}
	client := httpclient.NewHTTPClient(params)
	response, err := client.Get()
	if err != nil {
		log.Panic(err)
	}

	dict := new(xmlDictionary)
	err = xml.Unmarshal(response, dict)
	if err != nil {
		log.Panic(err)
	}
	return dict.Tags, dict.UIDs
}

func writeCopdingSchemesFile(uids []xmlDictionaryUID) {
	if FileExists(codingSchemesFile) {
		err := os.Remove(codingSchemesFile)
		if err != nil {
			log.Panic(err)
		}
	}
	f, err := os.Create(codingSchemesFile)
	if err != nil {
		log.Panic(err)
	}
	defer f.Close()

	f.WriteString("package codingscheme\n\n")

	codingSchemes := make([]string, 0)

	for _, uid := range uids {
		if uid.Type != "Coding Scheme" {
			continue
		}
		codingSchemes = append(codingSchemes, uid.Keyword)
		f.WriteString(fmt.Sprintf("// %s - (%s) %s\n", uid.Keyword, uid.UID, uid.Name))
		f.WriteString(fmt.Sprintf("var %s = &CodingScheme{\n", uid.Keyword))
		f.WriteString(fmt.Sprintf("  UID: \"%s\",\n", uid.UID))
		f.WriteString(fmt.Sprintf("  Name: \"%s\",\n", uid.Keyword))

		uid.Name = strings.ReplaceAll(uid.Name, " (Retired)", "")
		f.WriteString(fmt.Sprintf("  Description: \"%s\",\n", uid.Name))
		f.WriteString(fmt.Sprintf("  Type: \"%s\",\n", uid.Type))
		f.WriteString("}\n\n")
	}

	f.WriteString("var codingSchemes = []*CodingScheme{\n")
	for _, cs := range codingSchemes {
		f.WriteString(fmt.Sprintf("  %s,\n", cs))
	}
	f.WriteString("}\n")
	f.Sync()
}

func writeDicomTags(tags []xmlDictionaryTag) {
	if FileExists(dicomTagsFile) {
		err := os.Remove(dicomTagsFile)
		if err != nil {
			log.Panic(err)
		}
	}
	f, err := os.Create(dicomTagsFile)
	if err != nil {
		log.Panic(err)
	}
	defer f.Close()

	f.WriteString("package tags\n\n")

	dicomTags := make([]string, 0)

	for _, tag := range tags {
		if strings.Contains(tag.Group, "x") || strings.Contains(tag.Element, "x") {
			continue
		}
		dicomTags = append(dicomTags, tag.Keyword)
		f.WriteString(fmt.Sprintf("// %s - (%s,%s) %s\n", tag.Keyword, tag.Group, tag.Element, tag.Name))
		f.WriteString(fmt.Sprintf("var %s = &Tag{\n", tag.Keyword))
		f.WriteString(fmt.Sprintf("  Group: 0x%s,\n", tag.Group))
		f.WriteString(fmt.Sprintf("  Element: 0x%s,\n", tag.Element))
		f.WriteString(fmt.Sprintf("  VR: \"%s\",\n", tag.VR))
		f.WriteString(fmt.Sprintf("  VM: \"%s\",\n", tag.VM))
		f.WriteString(fmt.Sprintf("  Name: \"%s\",\n", tag.Keyword))

		tag.Name = strings.ReplaceAll(tag.Name, " (Trial)", "")
		tag.Name = strings.ReplaceAll(tag.Name, " (Retired)", "")
		f.WriteString(fmt.Sprintf("  Description: \"%s\",\n", tag.Name))
		f.WriteString("}\n\n")
	}

	f.WriteString("var tags = []*Tag{\n")
	for _, tag := range dicomTags {
		f.WriteString(fmt.Sprintf("  %s,\n", tag))
	}
	f.WriteString("}\n")

	f.Sync()
}

func writeSOPClassesFile(uids []xmlDictionaryUID) {
	if FileExists(sopClassesFile) {
		err := os.Remove(sopClassesFile)
		if err != nil {
			log.Panic(err)
		}
	}
	f, err := os.Create(sopClassesFile)
	if err != nil {
		log.Panic(err)
	}
	defer f.Close()

	f.WriteString("package sopclass\n\n")

	sopClasses := make([]string, 0)

	for _, uid := range uids {
		if uid.Type != "SOP Class" && uid.Type != "Application Context Name" {
			continue
		}

		sopClasses = append(sopClasses, uid.Keyword)
		f.WriteString(fmt.Sprintf("// %s - (%s) %s\n", uid.Keyword, uid.UID, uid.Name))
		f.WriteString(fmt.Sprintf("var %s = &SOPClass{\n", uid.Keyword))
		f.WriteString(fmt.Sprintf("  UID: \"%s\",\n", uid.UID))
		f.WriteString(fmt.Sprintf("  Name: \"%s\",\n", uid.Keyword))

		uid.Name = strings.ReplaceAll(uid.Name, " (Retired)", "")
		f.WriteString(fmt.Sprintf("  Description: \"%s\",\n", uid.Name))
		f.WriteString(fmt.Sprintf("  Type: \"%s\",\n", uid.Type))
		f.WriteString("}\n\n")
	}

	f.WriteString("var sopClasses = []*SOPClass{\n")
	for _, sopClass := range sopClasses {
		f.WriteString(fmt.Sprintf("  %s,\n", sopClass))
	}
	f.WriteString("}\n")
	f.Sync()
}

func writeTransferSyntaxesFile(uids []xmlDictionaryUID) {
	if FileExists(transferSyntaxesFile) {
		err := os.Remove(transferSyntaxesFile)
		if err != nil {
			log.Panic(err)
		}
	}
	f, err := os.Create(transferSyntaxesFile)
	if err != nil {
		log.Panic(err)
	}
	defer f.Close()

	f.WriteString("package transfersyntax\n\n")

	transferSyntaxes := make([]string, 0)

	for _, uid := range uids {
		if uid.Type != "Transfer Syntax" {
			continue
		}

		transferSyntaxes = append(transferSyntaxes, uid.Keyword)
		f.WriteString(fmt.Sprintf("// %s - (%s) %s\n", uid.Keyword, uid.UID, uid.Name))
		f.WriteString(fmt.Sprintf("var %s = &TransferSyntax{\n", uid.Keyword))
		f.WriteString(fmt.Sprintf("  UID: \"%s\",\n", uid.UID))
		f.WriteString(fmt.Sprintf("  Name: \"%s\",\n", uid.Keyword))

		uid.Name = strings.ReplaceAll(uid.Name, " (Retired)", "")
		description := strings.Split(uid.Name, ":")
		f.WriteString(fmt.Sprintf("  Description: \"%s\",\n", description[0]))
		f.WriteString(fmt.Sprintf("  Type: \"%s\",\n", uid.Type))
		f.WriteString("}\n\n")
	}

	f.WriteString("var transferSyntaxes = []*TransferSyntax{\n")
	for _, ts := range transferSyntaxes {
		f.WriteString(fmt.Sprintf("  %s,\n", ts))
	}
	f.WriteString("}\n")
	f.Sync()
}

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
