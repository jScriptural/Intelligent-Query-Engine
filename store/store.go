package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"context"
	"github.com/google/uuid"
	"intelliqe/internal/models"
	"log"
	_ "modernc.org/sqlite"
	"net/url"
	//"os"
	"strings"
	"errors"
	"time"
	//"strconv"
	"embed"
	//"io/fs"
)

//go:embed seed_profiles.json
var seedFS embed.FS

type DBHandler struct {
	DB *sql.DB
}

type ProfileData struct {
	Profiles []models.Profile `json:"profiles"`
}

func NewDBHandler(dbPath string) *DBHandler {
	log.Println("Setting up store")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("pinging db")
	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	log.Println("creating table in db")
	if err := createSchema(db); err != nil {
		log.Println("error creating schema")
		log.Fatal(err)
	}

	log.Println("seeding db")
	if err := seedDB(db, "seed_profiles.json"); err != nil {
		log.Fatal(err)
	}

	log.Println("Store is ready")
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
	log.Println("open seedfile")
	f, err := seedFS.Open(seedfile)
	if err != nil {
		return err
	}
	defer f.Close()

	p := ProfileData{}
	decoder := json.NewDecoder(f)

	log.Println("decode seedfile")
	if err := decoder.Decode(&p); err != nil {
		return err
	}
	log.Printf("profiles: %#v", p)

	log.Println("init tx: ready to write to db")
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

	log.Println("sql stmt prepared")
	stmt, err := tx.Prepare(s)
	if err != nil {
		return err
	}
	defer stmt.Close()
	log.Println("sql stmt prepared")

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

	log.Println("db seeded")
	return tx.Commit()
}

func (d *DBHandler) GetProfiles(ctx context.Context, q url.Values, page,limit int) ([]*models.Profile, int, error) {
	var (
		wc strings.Builder
		qc strings.Builder
	)
	
	qc.WriteString(`SELECT id,name,gender,gender_probability,age,age_group,country_id,country_name,country_probability,created_at
FROM profile`)
wc.WriteString(" WHERE 1=1")

	var args []any
	for k, v := range q {
		//k = strings.ToLower(k)
		val := v[0];
		switch {
		case k == "min_age":
			wc.WriteString(" AND age >= ?")
			args = append(args,val)
		case k == "max_age":
			wc.WriteString(" AND age <= ?")
			args = append(args,val)
		case k == "min_gender_probability":
			wc.WriteString(" AND gender_probability >= ?")
			args = append(args,val)
		case k == "gender":
			wc.WriteString(" AND gender = ?")
		  val = strings.ToLower(val)
			args = append(args,val)
		case k == "country_id":
			wc.WriteString(" AND country_id = ?")
			args = append(args,strings.ToUpper(val))
		case k == "age_group":
			wc.WriteString(" AND age_group = ?")
		  val = strings.ToLower(val)
			args = append(args,val)
		case k == "min_country_probability":
			wc.WriteString(" AND country_probability >= ?");
			args = append(args,val);
		}
	}

	totalCount := 0;
	sqlCount := "SELECT COUNT(*) FROM profile"+wc.String();

	err := d.DB.QueryRowContext(ctx,sqlCount,args...).Scan(&totalCount);
	if err != nil {
		return nil,0,fmt.Errorf("GetProfiles: %w",err);
	}


	qc.WriteString(wc.String());
	if q.Has("sort_by") && q.Has("order") {
		sort_by := strings.ToLower(q.Get("sort_by"));
		order := q.Get("order");
		s := fmt.Sprintf(" ORDER BY %s %s",sort_by,order)
		qc.WriteString(s)
	}



	offset := (page-1) * limit;
	qc.WriteString(fmt.Sprintf(" LIMIT %v OFFSET %v",limit, offset))

	qc.WriteString(";")
	stmt := qc.String();

	rows,err := d.DB.QueryContext(ctx,stmt,args...);
	if err != nil {
		if errors.Is(err, sql.ErrNoRows){
			return nil,0,fmt.Errorf("GetProfiles: %w",models.ErrNotFound);
		}
	
		return nil,0,fmt.Errorf("GetProfiles: %w",err);
	}
	defer rows.Close();

	p := []*models.Profile{}
	for rows.Next() {
		t := models.Profile{};
		rows.Scan(
			&t.ID,
			&t.Name,
			&t.Gender,
			&t.GenderProbability,
			&t.Age,
			&t.AgeGroup,
			&t.CountryID,
			&t.CountryName,
			&t.CountryProbability,
			&t.CreatedAt,
		)
		p = append(p,&t);
	}
	if rows.Err() != nil {
		log.Printf("Err: %v",err)
		return nil,0,fmt.Errorf("GetProfiles: %w",rows.Err);
	}



	if limit > totalCount {
		q.Set("limit",fmt.Sprintf("%v",totalCount))
	}
	return p,totalCount,nil
}
