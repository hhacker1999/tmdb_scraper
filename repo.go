package main

import (
	"database/sql"
	"log"
)

type Repo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) *Repo {
	return &Repo{
		db: db,
	}
}

func (r *Repo) CreateDb() error {
	_, err := r.db.Exec(`create table if not exists details (
    id serial primary key,
    tmdb_id int not null,
    data jsonb not null,
    type varchar(10) not null
    )`)
	if err != nil {
		log.Println("Error creating details table", err)
		return err
	}

	_, err = r.db.Exec(`create table if not exists movie_progress (
    id serial primary key,
    progress int
    )`)
	if err != nil {
		log.Println("Error creating movie_progress table", err)
		return err
	}

	_, err = r.db.Exec(`create table if not exists show_progress (
    id serial primary key,
    progress int
    )`)
	if err != nil {
		log.Println("Error creating show_progress table", err)
		return err
	}

	_, err = r.db.Exec(`create table if not exists failed (
    id serial primary key,
    type varchar(10) not null,
    tmdb_id int not null,
    error text not null
    )`)
	if err != nil {
		log.Println("Error creating failed table", err)
		return err
	}

	return nil
}

func (r *Repo) StoreDetails(id int, details []byte, tp string) error {
	_, err := r.db.Exec(
		`insert into details (tmdb_id, type, data) values($1, $2, $3)`,
		id,
		tp,
		details,
	)
	return err
}

func (r *Repo) UpdateMovieProgress(progress int) error {
	_, err := r.db.Exec(
		`insert into movie_progress (id, progress) values($1, $2) on conflict (id) do update set progress = excluded.progress`,
		1,
		progress,
	)
	return err
}

func (r *Repo) UpdateShowProgress(progress int) error {
	_, err := r.db.Exec(
		`insert into show_progress (id, progress) values($1, $2) on conflict (id) do update set progress = excluded.progress`,
		1,
		progress,
	)
	return err
}

func (r *Repo) InsertError(id int, tp string, er string) error {
	_, err := r.db.Exec(
		`insert into failed (tmdb_id, error, type) values($1, $2, $3)`,
		id,
		er,
		tp,
	)
	if err != nil {
		log.Println("Error storing failed", err)
	}
	return err
}

func (r *Repo) GetMovieProgress() (int, error) {
	var res int
	row := r.db.QueryRow(`select progress from movie_progress where id = 1`)
	err := row.Scan(&res)
  if err == sql.ErrNoRows {
    return 0 , nil
  }
	return res, err
}

func (r *Repo) GetShowProgress() (int, error) {
	var res int
	row := r.db.QueryRow(`select progress from show_progress where id = 1`)
	err := row.Scan(&res)
  if err == sql.ErrNoRows {
    return 0 , nil
  }
	return res, err
}
