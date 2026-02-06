package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type MovieCrwaler struct {
	usecase        *Usecase
	at             string
	repo           *Repo
	changeSyncPage int
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
		index, _, err := m.GetMovieProgress()
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
				m.repo.UpdateMovieProgress(v)
			} else {
				bt, err := json.Marshal(details)
				if err != nil {
					fmt.Printf("Error marhsalling movie data %d %v\n", v, err)
					m.repo.InsertError(v, "movie", err.Error())
					m.repo.UpdateMovieProgress(v)
				} else {
					err := m.repo.StoreDetails(v, bt, "movie")
					if err != nil {
						fmt.Println("Error storing data in db")
						m.repo.InsertError(v, "movie", err.Error())
						m.repo.UpdateMovieProgress(v)
						continue
					}
					err = m.repo.UpdateMovieProgress(v)
					if err != nil {
						fmt.Println("Error storing movie progress", err)
					}
					fmt.Println("Movie details stored for", v)

				}
			}
		}
	}

	return nil
}

func (m *MovieCrwaler) StartFailedJob(ctx context.Context) error {
	failed, err := m.repo.GetFailed(true)
	if err != nil {
		return err
	}
	fmt.Printf("Found %d failed movies\n", len(failed))
	for _, v := range failed {
		select {
		case <-ctx.Done():
			return nil
		default:
			details, err := m.usecase.GetMovieDetails(fmt.Sprintf("%d", v), m.at)
			if err != nil {
				fmt.Println("Error getting movie details for ", v, err)
				continue
			}
			bd, err := json.Marshal(details)
			if err != nil {
				fmt.Println("Error unmarshalling movie details for ", v, err)
				continue
			}
			err = m.repo.StoreDetails(v, bd, "movie")
			if err != nil {
				fmt.Println("Error storing movie details for ", v, err)
				continue
			}
			m.repo.RemoveFailed(true, v)
		}
	}

	return nil
}

func (m *MovieCrwaler) StartChangesSyncJob(ctx context.Context, days int) error {
	m.changeSyncPage = 0
	format := "2006-01-02"
	end := time.Now()
	start := end.AddDate(0, 0, -days)
	endDate := end.Format(format)
	startDate := start.Format(format)
	failed, err := m.usecase.GetMovieChanges(1, startDate, endDate, true, m.at)
	if err != nil {
		return err
	}
	fmt.Println("Found total movie changes ", failed.TotalResults)
	m.changeSyncPage = 1
	for _, res := range failed.Results {
		v := res.ID
		if res.Adult == nil {
			continue
		}
		details, err := m.usecase.GetMovieDetails(fmt.Sprintf("%d", v), m.at)
		if err != nil {
			fmt.Println("Error getting movie details for ", v, err)
		} else {
			bd, err := json.Marshal(details)
			if err != nil {
				fmt.Println("Error unmarshalling movie details for ", v, err)
			} else {
				err = m.repo.StoreDetails(v, bd, "movie")
				if err != nil {
					fmt.Println("Error storing movie details for ", v, err)
				}
			}
		}
	}
	for i := 2; i <= failed.TotalPages; i++ {
		select {
		case <-ctx.Done():
			return nil
		default:
			failed, err = m.usecase.GetMovieChanges(i, startDate, endDate, true, m.at)
			if err != nil {
				continue
			}
			m.changeSyncPage = i
		inner:
			for _, res := range failed.Results {
				v := res.ID
				if res.Adult == nil {
					continue
				}
				details, err := m.usecase.GetMovieDetails(fmt.Sprintf("%d", v), m.at)
				if err != nil {
					fmt.Println("Error getting movie details for ", v, err)
					continue inner
				}
				bd, err := json.Marshal(details)
				if err != nil {
					fmt.Println("Error unmarshalling movie details for ", v, err)
					continue inner
				}
				err = m.repo.StoreDetails(v, bd, "movie")
				if err != nil {
					fmt.Println("Error storing movie details for ", v, err)
					continue inner
				}
			}
		}
	}
	return nil
}

func (m *MovieCrwaler) GetMovieProgress() (int, int, error) {
	mProgress, err := m.repo.GetMovieProgress()
	return mProgress, m.changeSyncPage, err
}
