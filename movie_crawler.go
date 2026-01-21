package main

import (
	"encoding/json"
	"fmt"
)

type MovieCrwaler struct {
	usecase   *Usecase
	currIndex int
	movies    int
	at        string
	repo      *Repo
}

func NewMovieCrawler(usecase *Usecase, at string, repo *Repo) *MovieCrwaler {
	return &MovieCrwaler{
		usecase:   usecase,
		currIndex: 1,
		movies:    1500000,
		at:        at,
		repo:      repo,
	}
}

func (m *MovieCrwaler) Start() error {
	index, err := m.repo.GetMovieProgress()
	if err != nil {
		return err
	}
	fmt.Println("Starting crawler for movies from index", index)
	for i := index + 1; i <= m.movies; i++ {
		v := i
		fmt.Println("Geting info for movie", v)
		details, err := m.usecase.GetMovieDetails(fmt.Sprintf("%d", v), m.at)
		if err != nil {
			fmt.Println("Error getting movie details for", v)
			m.repo.InsertError(v, "movie", err.Error())
		} else {
			bt, err := json.Marshal(details)
			if err != nil {
				fmt.Printf("Error marhsalling movie data %d %v\n", v, err)
				m.repo.InsertError(v, "movie", err.Error())
			} else {
				err := m.repo.StoreDetails(v, bt, "movie")
				if err != nil {
					fmt.Println("Error storing data in db")
					m.repo.InsertError(v, "movie", err.Error())
					continue
				}
				err = m.repo.UpdateMovieProgress(v)
				if err != nil {
					fmt.Println("Error storing movie progress", err)
				} else {
					fmt.Println("Movie details stored for", v)
				}
			}
		}
	}

	return nil
}
