package main

import (
	"context"
	"encoding/json"
	"fmt"
)

type ShowCrwaler struct {
	usecase *Usecase
	at      string
	repo    *Repo
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
		index, err := m.GetShowProgress()
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
			} else {
				bt, err := json.Marshal(details)
				if err != nil {
					fmt.Printf("Error marhsalling show data %d %v\n", v, err)
					m.repo.InsertError(v, "show", err.Error())
				} else {
					err := m.repo.StoreDetails(v, bt, "show")
					if err != nil {
						fmt.Println("Error storing data in db")
						m.repo.InsertError(v, "show", err.Error())
						continue
					}
					err = m.repo.UpdateShowProgress(v)
					if err != nil {
						fmt.Println("Error storing show progress", err)
					} else {
						fmt.Println("Show details stored for", v)
					}
				}
			}
		}

	}

	return nil
}

func (m *ShowCrwaler) GetShowProgress() (int, error) {
	return m.repo.GetShowProgress()
}
