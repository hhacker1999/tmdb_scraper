package main

import (
	"context"
	"fmt"
	"time"
)

type ScrapeStats struct {
	ShowCrawling         bool       `json:"show_crawling"`
	MovieCrawling        bool       `json:"movie_crawling"`
	IMDBWorking          bool       `json:"imdb_working"`
	MovieProgress        int        `json:"movie_progress"`
	ShowProgress         int        `json:"show_progress"`
	LastMovieCrwalerTime *time.Time `json:"last_movie_crwaler_time,omitempty"`
	LastShowCrwalerTime  *time.Time `json:"last_show_crwaler_time,omitempty"`
	LastIMDBSyncTime     *time.Time `json:"last_imdb_sync_time,omitempty"`
}

type ScrapeManager struct {
	showC        *ShowCrwaler
	movieC       *MovieCrwaler
	imdbI        *IMDBImporter
	showCancel   context.CancelFunc
	movieCancel  context.CancelFunc
	imdbCancel   context.CancelFunc
	showWorking  bool
	movieWorking bool
	imdbWorking  bool
	showTime     *time.Time
	movieTime    *time.Time
	imdbTime     *time.Time
}

func NewScrapeManager(
	showC *ShowCrwaler,
	movieC *MovieCrwaler,
	imdbI *IMDBImporter,
) *ScrapeManager {
	return &ScrapeManager{
		showC:  showC,
		movieC: movieC,
		imdbI:  imdbI,
	}
}

func (m *ScrapeManager) GetStats() (ScrapeStats, error) {
	res := ScrapeStats{
		ShowCrawling:         m.showWorking,
		MovieCrawling:        m.movieWorking,
		IMDBWorking:          m.imdbWorking,
		LastMovieCrwalerTime: m.movieTime,
		LastIMDBSyncTime:     m.imdbTime,
		LastShowCrwalerTime:  m.showTime,
	}

	index, err := m.movieC.GetMovieProgress()
	if err != nil {
		fmt.Println("Error getting movie progress", err)
	}
	res.MovieProgress = index

	index, err = m.showC.GetShowProgress()
	if err != nil {
		fmt.Println("Error getting movie progress", err)
	}
	res.ShowProgress = index

	return res, nil
}

func (m *ScrapeManager) StartMovieSync(start int, end int, overwrite bool) error {
	if m.movieWorking {
		return fmt.Errorf("Movie sync is currently in progress")
	}
	go m.startMovieSyncInternal(start, end, overwrite)
	return nil
}

func (m *ScrapeManager) startMovieSyncInternal(start int, end int, overwrite bool) {
	m.movieWorking = true
	tm := time.Now()
	m.movieTime = &tm
	ctx, cFunc := context.WithCancel(context.Background())
	m.movieCancel = cFunc
	err := m.movieC.Start(ctx, start, end, overwrite)
	if err != nil {
		fmt.Println("Movie scraper errored out with", err)
	}
	if m.movieCancel != nil {
		m.movieCancel()
	}
	m.movieWorking = false
}

func (m *ScrapeManager) StartShowSync(start int, end int, overwrite bool) error {
	if m.showWorking {
		return fmt.Errorf("Show sync in currently in progress")
	}

	go m.startShowSyncInternal(start, end, overwrite)
	return nil
}

func (m *ScrapeManager) startShowSyncInternal(start int, end int, overwrite bool) {
	m.showWorking = true
	tm := time.Now()
	m.showTime = &tm
	ctx, cFunc := context.WithCancel(context.Background())
	m.showCancel = cFunc
	err := m.showC.Start(ctx, start, end, overwrite)
	if err != nil {
		fmt.Println("Show scraper errored out with", err)
	}
	if m.showCancel != nil {
		m.showCancel()
	}
	m.showWorking = false
}

func (m *ScrapeManager) StartIMDBSync() error {
	if m.imdbWorking {
		return fmt.Errorf("IMDB sync is currently in progress")
	}
	go m.startIMDBSyncInternal()
	return nil
}

func (m *ScrapeManager) startIMDBSyncInternal() {
	m.imdbWorking = true
	tm := time.Now()
	m.imdbTime = &tm
	ctx, cFunc := context.WithCancel(context.Background())
	m.imdbCancel = cFunc
	err := m.imdbI.Start(ctx)
	if err != nil {
		fmt.Println("Imdb scraper errored out with", err)
	}
	if m.imdbCancel != nil {
		m.imdbCancel()
	}
	m.imdbWorking = false
}

func (m *ScrapeManager) StopMovieScrape() {
	if m.movieCancel != nil {
		m.movieCancel()
		m.movieCancel = nil
	}
}

func (m *ScrapeManager) StopShowScrape() {
	if m.showCancel != nil {
		m.showCancel()
		m.showCancel = nil
	}
}

func (m *ScrapeManager) StopImdbScrape() {
	if m.imdbCancel != nil {
		m.imdbCancel()
		m.imdbCancel = nil
	}
}

func (m *ScrapeManager) ShutDown() {
	if m.movieCancel != nil {
		m.movieCancel()
	}
	if m.showCancel != nil {
		m.showCancel()
	}
	if m.imdbCancel != nil {
		m.imdbCancel()
	}
}
