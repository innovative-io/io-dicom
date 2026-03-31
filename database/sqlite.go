package database

import (
	"database/sql"

	"github.com/innovative-io/io-dicom/media"

	_ "modernc.org/sqlite"
)

type SQLite struct {
	db *sql.DB
}

func NewSQLiteDatabase(dbFileName string) (Database, error) {
	db, err := sql.Open("sqlite", dbFileName)
	if err != nil {
		return nil, err
	}

	return &SQLite{db: db}, nil
}

func (s *SQLite) AddPatient(dicomObject media.DICOMObject) error {
	return nil
}

func (s *SQLite) AddStudy(dicomObject media.DICOMObject) error {
	return nil
}

func (s *SQLite) AddSeries(dicomObject media.DICOMObject) error {
	return nil
}

func (s *SQLite) AddInstance(dicomObject media.DICOMObject) error {
	return nil
}

func (s *SQLite) AddDicom(dicomObject media.DICOMObject) error {
	return nil
}
