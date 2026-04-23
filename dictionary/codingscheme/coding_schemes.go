package codingscheme

// DCM - (1.2.840.10008.2.16.4) DICOM Controlled Terminology
var DCM = &CodingScheme{
  UID: "1.2.840.10008.2.16.4",
  Name: "DCM",
  Description: "DICOM Controlled Terminology",
  Type: "Coding Scheme",
}

// MA - (1.2.840.10008.2.16.5) Adult Mouse Anatomy Ontology
var MA = &CodingScheme{
  UID: "1.2.840.10008.2.16.5",
  Name: "MA",
  Description: "Adult Mouse Anatomy Ontology",
  Type: "Coding Scheme",
}

// UBERON - (1.2.840.10008.2.16.6) Uberon Ontology
var UBERON = &CodingScheme{
  UID: "1.2.840.10008.2.16.6",
  Name: "UBERON",
  Description: "Uberon Ontology",
  Type: "Coding Scheme",
}

// ITIS_TSN - (1.2.840.10008.2.16.7) Integrated Taxonomic Information System (ITIS) Taxonomic Serial Number (TSN)
var ITIS_TSN = &CodingScheme{
  UID: "1.2.840.10008.2.16.7",
  Name: "ITIS_TSN",
  Description: "Integrated Taxonomic Information System (ITIS) Taxonomic Serial Number (TSN)",
  Type: "Coding Scheme",
}

// MGI - (1.2.840.10008.2.16.8) Mouse Genome Initiative (MGI)
var MGI = &CodingScheme{
  UID: "1.2.840.10008.2.16.8",
  Name: "MGI",
  Description: "Mouse Genome Initiative (MGI)",
  Type: "Coding Scheme",
}

// PUBCHEM_CID - (1.2.840.10008.2.16.9) PubChem Compound CID
var PUBCHEM_CID = &CodingScheme{
  UID: "1.2.840.10008.2.16.9",
  Name: "PUBCHEM_CID",
  Description: "PubChem Compound CID",
  Type: "Coding Scheme",
}

// DC - (1.2.840.10008.2.16.10) Dublin Core
var DC = &CodingScheme{
  UID: "1.2.840.10008.2.16.10",
  Name: "DC",
  Description: "Dublin Core",
  Type: "Coding Scheme",
}

// NYUMCCG - (1.2.840.10008.2.16.11) New York University Melanoma Clinical Cooperative Group
var NYUMCCG = &CodingScheme{
  UID: "1.2.840.10008.2.16.11",
  Name: "NYUMCCG",
  Description: "New York University Melanoma Clinical Cooperative Group",
  Type: "Coding Scheme",
}

// MAYONRISBSASRG - (1.2.840.10008.2.16.12) Mayo Clinic Non-radiological Images Specific Body Structure Anatomical Surface Region Guide
var MAYONRISBSASRG = &CodingScheme{
  UID: "1.2.840.10008.2.16.12",
  Name: "MAYONRISBSASRG",
  Description: "Mayo Clinic Non-radiological Images Specific Body Structure Anatomical Surface Region Guide",
  Type: "Coding Scheme",
}

// IBSI - (1.2.840.10008.2.16.13) Image Biomarker Standardisation Initiative
var IBSI = &CodingScheme{
  UID: "1.2.840.10008.2.16.13",
  Name: "IBSI",
  Description: "Image Biomarker Standardisation Initiative",
  Type: "Coding Scheme",
}

// RO - (1.2.840.10008.2.16.14) Radiomics Ontology
var RO = &CodingScheme{
  UID: "1.2.840.10008.2.16.14",
  Name: "RO",
  Description: "Radiomics Ontology",
  Type: "Coding Scheme",
}

// RADELEMENT - (1.2.840.10008.2.16.15) RadElement
var RADELEMENT = &CodingScheme{
  UID: "1.2.840.10008.2.16.15",
  Name: "RADELEMENT",
  Description: "RadElement",
  Type: "Coding Scheme",
}

// I11 - (1.2.840.10008.2.16.16) ICD-11
var I11 = &CodingScheme{
  UID: "1.2.840.10008.2.16.16",
  Name: "I11",
  Description: "ICD-11",
  Type: "Coding Scheme",
}

// UNS - (1.2.840.10008.2.16.17) Unified numbering system (UNS) for metals and alloys
var UNS = &CodingScheme{
  UID: "1.2.840.10008.2.16.17",
  Name: "UNS",
  Description: "Unified numbering system (UNS) for metals and alloys",
  Type: "Coding Scheme",
}

// RRID - (1.2.840.10008.2.16.18) Research Resource Identification
var RRID = &CodingScheme{
  UID: "1.2.840.10008.2.16.18",
  Name: "RRID",
  Description: "Research Resource Identification",
  Type: "Coding Scheme",
}

var codingSchemes = []*CodingScheme{
  DCM,
  MA,
  UBERON,
  ITIS_TSN,
  MGI,
  PUBCHEM_CID,
  DC,
  NYUMCCG,
  MAYONRISBSASRG,
  IBSI,
  RO,
  RADELEMENT,
  I11,
  UNS,
  RRID,
}
