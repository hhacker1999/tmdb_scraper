package main

import (
	"context"
	"fmt"
	"time"
)

type ScrapeStats struct {
	ShowCrawling             bool       `json:"show_crawling"`
	MovieCrawling            bool       `json:"movie_crawling"`
	IMDBWorking              bool       `json:"imdb_working"`
	MovieProgress            int        `json:"movie_progress"`
	ShowProgress             int        `json:"show_progress"`
	LastMovieCrwalerTime     *time.Time `json:"last_movie_crwaler_time,omitempty"`
	LastShowCrwalerTime      *time.Time `json:"last_show_crwaler_time,omitempty"`
	LastIMDBSyncTime         *time.Time `json:"last_imdb_sync_time,omitempty"`
	LastFailedSyncTimeMovie  *time.Time `json:"last_failed_sync_time_movie,omitempty"`
	LastChangesSyncTimeMovie *time.Time `json:"last_changes_sync_time_movie,omitempty"`
	LastFailedSyncTimeShow   *time.Time `json:"last_failed_sync_time_show,omitempty"`
	LastChangesSyncTimeShow  *time.Time `json:"last_changes_sync_time_show,omitempty"`
	FailedSyncingShow        bool       `json:"failed_syncing_show"`
	FailedSyncingMovie       bool       `json:"failed_syncing_movie"`
	ChangesSyncingShow       bool       `json:"changes_syncing_show"`
	ChangesSyncingMovie      bool       `json:"changes_syncing_movie"`
	ChangeSyncPageMovie      int        `json:"change_sync_page_movie"`
	ChangeSyncPageShow       int        `json:"change_sync_page_show"`
}

type ScrapeManager struct {
	showC                    *ShowCrwaler
	movieC                   *MovieCrwaler
	imdbI                    *IMDBImporter
	showCancel               context.CancelFunc
	movieCancel              context.CancelFunc
	imdbCancel               context.CancelFunc
	failMovieSyncCancel      context.CancelFunc
	failShowSyncCancel       context.CancelFunc
	changeMovieSyncCancel    context.CancelFunc
	changeShowSyncCancel     context.CancelFunc
	showWorking              bool
	movieWorking             bool
	imdbWorking              bool
	showTime                 *time.Time
	movieTime                *time.Time
	imdbTime                 *time.Time
	LastFailedSyncTimeMovie  *time.Time
	LastChangesSyncTimeMovie *time.Time
	LastFailedSyncTimeShow   *time.Time
	LastChangesSyncTimeShow  *time.Time
	FailedSyncingShow        bool
	FailedSyncingMovie       bool
	ChangesSyncingShow       bool
	ChangesSyncingMovie      bool
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
		ShowCrawling:             m.showWorking,
		MovieCrawling:            m.movieWorking,
		IMDBWorking:              m.imdbWorking,
		LastMovieCrwalerTime:     m.movieTime,
		LastIMDBSyncTime:         m.imdbTime,
		LastShowCrwalerTime:      m.showTime,
		LastFailedSyncTimeMovie:  m.LastFailedSyncTimeMovie,
		LastFailedSyncTimeShow:   m.LastFailedSyncTimeShow,
		LastChangesSyncTimeMovie: m.LastChangesSyncTimeMovie,
		LastChangesSyncTimeShow:  m.LastChangesSyncTimeShow,
		ChangesSyncingMovie:      m.ChangesSyncingMovie,
		ChangesSyncingShow:       m.ChangesSyncingShow,
		FailedSyncingShow:        m.FailedSyncingShow,
		FailedSyncingMovie:       m.FailedSyncingMovie,
	}

	index, moviePage, err := m.movieC.GetMovieProgress()
	if err != nil {
		fmt.Println("Error getting movie progress", err)
	}
	res.MovieProgress = index
	res.ChangeSyncPageMovie = moviePage

	index, showPage, err := m.showC.GetShowProgress()
	if err != nil {
		fmt.Println("Error getting movie progress", err)
	}
	res.ShowProgress = index
	res.ChangeSyncPageShow = showPage

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

func (m *ScrapeManager) StartFailSyncMovie() error {
	if m.FailedSyncingMovie {
		return fmt.Errorf("Fail sync is currently in progress")
	}
	go m.startFailSyncMovieInternal()
	return nil
}

func (m *ScrapeManager) startFailSyncMovieInternal() {
	m.FailedSyncingMovie = true
	tm := time.Now()
	m.LastFailedSyncTimeMovie = &tm
	ctx, cFunc := context.WithCancel(context.Background())
	m.failMovieSyncCancel = cFunc
	err := m.movieC.StartFailedJob(ctx)
	if err != nil {
		fmt.Println("Fail movie sync errored out with", err)
	}
	if m.failMovieSyncCancel != nil {
		m.failMovieSyncCancel()
	}
	m.FailedSyncingMovie = false
}

func (m *ScrapeManager) StartFailSyncShow() error {
	if m.FailedSyncingShow {
		return fmt.Errorf("Fail sync is currently in progress")
	}
	go m.startFailSyncshowInternal()
	return nil
}

func (m *ScrapeManager) startFailSyncshowInternal() {
	m.FailedSyncingShow = true
	tm := time.Now()
	m.LastFailedSyncTimeShow = &tm
	ctx, cFunc := context.WithCancel(context.Background())
	m.failShowSyncCancel = cFunc
	err := m.showC.StartFailedJob(ctx)
	if err != nil {
		fmt.Println("Fail show sync errored out with", err)
	}
	if m.failShowSyncCancel != nil {
		m.failShowSyncCancel()
	}
	m.FailedSyncingShow = false
}

func (m *ScrapeManager) StartChangeSyncShow(days int) error {
	if days == 0 {
		return fmt.Errorf("Invalid days")
	}
	if m.ChangesSyncingShow {
		return fmt.Errorf("Change sync is currently in progress")
	}
	go m.startChangeSyncshowInternal(days)
	return nil
}

func (m *ScrapeManager) startChangeSyncshowInternal(days int) {
	m.ChangesSyncingShow = true
	tm := time.Now()
	m.LastChangesSyncTimeShow = &tm
	ctx, cFunc := context.WithCancel(context.Background())
	m.changeShowSyncCancel = cFunc
	err := m.showC.StartChangesSyncJob(ctx, days)
	if err != nil {
		fmt.Println("Change show sync errored out with", err)
	}
	if m.changeShowSyncCancel != nil {
		m.changeShowSyncCancel()
	}
	m.ChangesSyncingShow = false
}

func (m *ScrapeManager) StartChangeSyncMovie(days int) error {
	if days == 0 {
		return fmt.Errorf("Invalid days")
	}
	if m.ChangesSyncingMovie {
		return fmt.Errorf("Change sync is currently in progress")
	}
	go m.startChangeSyncMovieInternal(days)
	return nil
}

func (m *ScrapeManager) startChangeSyncMovieInternal(days int) {
	m.ChangesSyncingMovie = true
	tm := time.Now()
	m.LastChangesSyncTimeMovie = &tm
	ctx, cFunc := context.WithCancel(context.Background())
	m.changeMovieSyncCancel = cFunc
	err := m.movieC.StartChangesSyncJob(ctx, days)
	if err != nil {
		fmt.Println("Change Movie sync errored out with", err)
	}
	if m.changeMovieSyncCancel != nil {
		m.changeMovieSyncCancel()
	}
	m.ChangesSyncingMovie = false
}

func (m *ScrapeManager) StopFailSyncMovie() {
	if m.failMovieSyncCancel != nil {
		m.failMovieSyncCancel()
		m.failMovieSyncCancel = nil
	}
}

func (m *ScrapeManager) StopFailSyncShow() {
	if m.failShowSyncCancel != nil {
		m.failShowSyncCancel()
		m.failShowSyncCancel = nil
	}
}

func (m *ScrapeManager) StopChangeSyncShow() {
	if m.changeShowSyncCancel != nil {
		m.changeShowSyncCancel()
		m.changeShowSyncCancel = nil
	}
}

func (m *ScrapeManager) StopChangeSyncMovie() {
	if m.changeMovieSyncCancel != nil {
		m.changeMovieSyncCancel()
		m.changeMovieSyncCancel = nil
	}
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
	if m.changeMovieSyncCancel != nil {
		m.changeMovieSyncCancel()
		m.changeMovieSyncCancel = nil
	}
	if m.changeShowSyncCancel != nil {
		m.changeShowSyncCancel()
		m.changeShowSyncCancel = nil
	}
	if m.failShowSyncCancel != nil {
		m.failShowSyncCancel()
		m.failShowSyncCancel = nil
	}
	if m.failMovieSyncCancel != nil {
		m.failMovieSyncCancel()
		m.failMovieSyncCancel = nil
	}
}
