package main

import (
	"encoding/json"
	"fmt"
)

type ShowCrwaler struct {
	usecase   *Usecase
	currIndex int
	shows     int
	at        string
	repo      *Repo
}

func NewShowCrawler(usecase *Usecase, at string, repo *Repo) *ShowCrwaler {
	return &ShowCrwaler{
		usecase:   usecase,
		currIndex: 1,
		shows:     350000,
		at:        at,
		repo:      repo,
	}
}

func (m *ShowCrwaler) Start() error {
	index, err := m.repo.GetShowProgress()
	if err != nil {
		return err
	}
	fmt.Println("Starting crawler for shows from index", index)
	for i := index + 1; i <= m.shows; i++ {
		v := i
		details, err := m.usecase.GetShowDetails(fmt.Sprintf("%d", v), m.at)
		if err != nil {
			fmt.Println("Error getting show details for", v)
			m.repo.InsertError(v, "show", err.Error())
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
					// fmt.Println("Show details stored for", v)
				}
			}
		}

	}

	return nil
}
