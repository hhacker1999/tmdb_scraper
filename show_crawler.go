package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type ShowCrwaler struct {
	usecase        *Usecase
	at             string
	repo           *Repo
	changeSyncPage int
}

func NewShowCrawler(usecase *Usecase, at string, repo *Repo) *ShowCrwaler {
	return &ShowCrwaler{
		usecase: usecase,
		at:      at,
		repo:    repo,
	}
}

func (m *ShowCrwaler) Start(ctx context.Context, start int, end int, overwrite bool) error {
	if end == 0 {
		end = 350000
	}
	if start == 0 {
		index, _, err := m.GetShowProgress()
		if err != nil {
			return err
		}
		start = index + 1
	}
	fmt.Println("Starting crawler for shows from index", start)
	for i := start; i <= end; i++ {
		select {
		case <-ctx.Done():
			return nil
		default:
			v := i

			if !overwrite {
				exists, err := m.repo.ItemExists("show", v)
				if err != nil {
					fmt.Println("Error getting item exists", err)
				}
				if exists {
					fmt.Println("Skipping item since its found")
					err = m.repo.UpdateShowProgress(v)
					if err != nil {
						fmt.Println("Error storing show progress", err)
					}
					continue
				}
			}

			exists, err := m.repo.NotFoundExists("show", v)
			if err != nil {
				fmt.Println("Error getting not found", err)
			}
			if exists {
				fmt.Println("Skipping item since it does not exists")
				err = m.repo.UpdateShowProgress(v)
				if err != nil {
					fmt.Println("Error storing show progress", err)
				}
				continue
			}

			details, err := m.usecase.GetShowDetails(fmt.Sprintf("%d", v), m.at)
			if err != nil {
				fmt.Println("Error getting show details for", v)
				if err.Error() == "not found" {
					m.repo.InsertNotFound(v, "show")
				} else {
					m.repo.InsertError(v, "show", err.Error())
				}
				err = m.repo.UpdateShowProgress(v)
			} else {
				bt, err := json.Marshal(details)
				if err != nil {
					fmt.Printf("Error marhsalling show data %d %v\n", v, err)
					m.repo.InsertError(v, "show", err.Error())
					err = m.repo.UpdateShowProgress(v)
				} else {
					err := m.repo.StoreDetails(v, bt, "show")
					if err != nil {
						fmt.Println("Error storing data in db")
						m.repo.InsertError(v, "show", err.Error())
						err = m.repo.UpdateShowProgress(v)
						continue
					}
					err = m.repo.UpdateShowProgress(v)
					if err != nil {
						fmt.Println("Error storing show progress", err)
					}
					fmt.Println("Show details stored for", v)
				}
			}
		}

	}

	return nil
}

func (m *ShowCrwaler) StartFailedJob(ctx context.Context) error {
	failed, err := m.repo.GetFailed(false)
	if err != nil {
		return err
	}
	fmt.Printf("Found %d failed shows\n", len(failed))
	for _, v := range failed {
		select {
		case <-ctx.Done():
			return nil
		default:
			details, err := m.usecase.GetShowDetails(fmt.Sprintf("%d", v), m.at)
			if err != nil {
				fmt.Println("Error getting show details for ", v, err)
				continue
			}
			bd, err := json.Marshal(details)
			if err != nil {
				fmt.Println("Error unmarshalling show details for ", v, err)
				continue
			}
			err = m.repo.StoreDetails(v, bd, "show")
			if err != nil {
				fmt.Println("Error storing show details for ", v, err)
				continue
			}
			m.repo.RemoveFailed(false, v)
		}
	}

	return nil
}

func (m *ShowCrwaler) StartChangesSyncJob(ctx context.Context, days int) error {
	m.changeSyncPage = 0
	format := "2006-01-02"
	end := time.Now()
	start := end.AddDate(0, 0, -days)
	endDate := end.Format(format)
	startDate := start.Format(format)
	failed, err := m.usecase.GetMovieChanges(1, startDate, endDate, false, m.at)
	if err != nil {
		return err
	}
	fmt.Println("Found total show changes ", failed.TotalResults)
	m.changeSyncPage = 1
	for _, res := range failed.Results {
		v := res.ID
		if res.Adult == nil {
			continue
		}
		details, err := m.usecase.GetShowDetails(fmt.Sprintf("%d", v), m.at)
		if err != nil {
			fmt.Println("Error getting show details for ", v, err)
		} else {
			bd, err := json.Marshal(details)
			if err != nil {
				fmt.Println("Error unmarshalling show details for ", v, err)
			} else {
				err = m.repo.StoreDetails(v, bd, "show")
				if err != nil {
					fmt.Println("Error storing show details for ", v, err)
				}
			}
		}
	}
	for i := 2; i <= failed.TotalPages; i++ {
		select {
		case <-ctx.Done():
			return nil
		default:
			failed, err = m.usecase.GetMovieChanges(i, startDate, endDate, false, m.at)
			if err != nil {
				continue
			}
			m.changeSyncPage = i
		inner:
			for _, res := range failed.Results {
				v := res.ID
				if res.Adult == nil {
					continue inner
				}
				details, err := m.usecase.GetShowDetails(fmt.Sprintf("%d", v), m.at)
				if err != nil {
					fmt.Println("Error getting show details for ", v, err)
					continue inner
				}
				bd, err := json.Marshal(details)
				if err != nil {
					fmt.Println("Error unmarshalling show details for ", v, err)
					continue inner
				}
				err = m.repo.StoreDetails(v, bd, "show")
				if err != nil {
					fmt.Println("Error storing show details for ", v, err)
					continue inner
				}
			}
		}
	}
	return nil
}

func (m *ShowCrwaler) GetShowProgress() (int, int, error) {
	mProgress, err := m.repo.GetShowProgress()
	return mProgress, m.changeSyncPage, err
}
