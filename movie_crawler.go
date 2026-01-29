package main

import (
	"context"
	"encoding/json"
	"fmt"
)

type MovieCrwaler struct {
	usecase *Usecase
	at      string
	repo    *Repo
}

func NewMovieCrawler(usecase *Usecase, at string, repo *Repo) *MovieCrwaler {
	return &MovieCrwaler{
		usecase: usecase,
		at:      at,
		repo:    repo,
	}
}

func (m *MovieCrwaler) Start(ctx context.Context, start int, end int, overwrite bool) error {
	if end == 0 {
		end = 2000000
	}
	if start == 0 {
		index, err := m.GetMovieProgress()
		if err != nil {
			return err
		}
		start = index + 1
	}
	fmt.Println("Starting crawler for movies from index", start)
	for i := start; i <= end; i++ {
		select {
		case <-ctx.Done():
			return nil
		default:
			v := i

			if !overwrite {
				exists, err := m.repo.ItemExists("movie", v)
				if err != nil {
					fmt.Println("Error getting item exists", err)
				}
				if exists {
					fmt.Println("Skipping item since its found")
					err = m.repo.UpdateMovieProgress(v)
					if err != nil {
						fmt.Println("Error storing movie progress", err)
					}
					continue
				}
			}

			exists, err := m.repo.NotFoundExists("movie", v)
			if err != nil {
				fmt.Println("Error getting not found", err)
			}
			if exists {
				fmt.Println("Skipping item since it does not exists")
				err = m.repo.UpdateMovieProgress(v)
				if err != nil {
					fmt.Println("Error storing movie progress", err)
				}
				continue
			}

			details, err := m.usecase.GetMovieDetails(fmt.Sprintf("%d", v), m.at)
			if err != nil {
				fmt.Println("Error getting movie details for", v)
				if err.Error() == "not found" {
					m.repo.InsertNotFound(v, "movie")
				} else {
					m.repo.InsertError(v, "movie", err.Error())
				}
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
	}

	return nil
}

func (m *MovieCrwaler) GetMovieProgress() (int, error) {
	return m.repo.GetMovieProgress()
}
