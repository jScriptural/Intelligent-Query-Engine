package store

import (
	"database/sql"
	"encoding/json"
	//"fmt"
	"github.com/google/uuid"
	"intelliqe/internal/models"
	"log"
	_ "modernc.org/sqlite"
	"os"
	"time"
)

type DBHandler struct {
	DB *sql.DB
}

type ProfileData struct {
	Profiles []models.Profile `json:"profiles"`
}

func NewDBHandler(dbPath string) *DBHandler {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	if err := createSchema(db); err != nil {
		log.Fatal(err)
	}

	if err := seedDB(db, "./store/seed_profiles.json"); err != nil {
		log.Fatal(err)
	}

	return &DBHandler{DB: db}
}

func createSchema(db *sql.DB) error {
	schema := `CREATE TABLE IF NOT EXISTS profile (
		id BLOB PRIMARY KEY,
		name TEXT UNIQUE NOT NULL,
		gender TEXT,
		gender_probability FLOAT,
		age INTEGER,
		age_group TEXT,
		country_id TEXT NOT NULL CHECK(LENGTH(country_id) = 2),
		country_name TEXT,
		country_probability REAL,
		created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	);

	CREATE INDEX IF NOT EXISTS idx_profile_age_group ON profile(age_group);
	CREATE INDEX IF NOT EXISTS idx_profile_age ON profile(age);
	CREATE INDEX IF NOT EXISTS idx_profile_gender ON profile(gender);
	CREATE INDEX IF NOT EXISTS idx_profile_country_id ON profile(country_id);
	`

	_, err := db.Exec(schema)
	return err
}

func seedDB(db *sql.DB, seedfile string) error {
	f, err := os.Open(seedfile)
	if err != nil {
		return err
	}
	defer f.Close()

	p := ProfileData{}
	decoder := json.NewDecoder(f)

	if err := decoder.Decode(&p); err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	s := `INSERT INTO profile (
		id,name,gender,gender_probability,
		age,age_group,country_id,country_name,
		country_probability,created_at) 
		VALUES (?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(name) DO NOTHING;`

	stmt, err := tx.Prepare(s)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, v := range p.Profiles {
		id, _ := uuid.NewV7()
		d := time.Now().UTC().Format(time.RFC3339)
		_, err := stmt.Exec(
			id,
			v.Name,
			v.Gender,
			v.GenderProbability,
			v.Age,
			v.AgeGroup,
			v.CountryID,
			v.CountryName,
			v.CountryProbability,
			d,
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	tx.Commit()
	return nil
}
