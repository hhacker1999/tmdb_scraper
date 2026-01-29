package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/lib/pq"
)

func main() {
	dsn := os.Getenv("DB_URL")
	at := os.Getenv("TMDB_AT")
	url := os.Getenv("TMDB_BASE_URL")
	dataDir := os.Getenv("DATA_DIR")
	// dsn = "postgres://pg:pg@192.168.1.50:5555/tmdb?sslmode=disable"
	// at = "eyJhbGciOiJIUzI1NiJ9.eyJhdWQiOiI2NWJjYTJhN2NhODdkNTZkZGZlMDgyZDAzOWNiZjk1ZiIsIm5iZiI6MTY1MDA0MzA3My4wMTksInN1YiI6IjYyNTlhOGMxZWNhZWY1MTVmZjY3OGY3MyIsInNjb3BlcyI6WyJhcGlfcmVhZCJdLCJ2ZXJzaW9uIjoxfQ.EppXuTBWBa1uXJgfie3m7lKAEpspRwnc_aHr33UBkHU"
	// dataDir = "./"
	// url = "https://api.themoviedb.org/3"

	log.Println("Connecting to db")
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatal(err)
	}

	log.Println("Connected to Postgres.")
	client := NewClient()
	uc := NewUsecase(url, client)

	repo := NewRepo(db)
	err = repo.CreateDb()
	if err != nil {
		log.Fatal(err)
		return
	}

	mc := NewMovieCrawler(
		uc,
		at,
		repo,
	)
	sc := NewShowCrawler(
		uc,
		at,
		repo,
	)

	imdbI := NewIMDbImporter(db, dataDir)

	manager := NewScrapeManager(sc, mc, imdbI)

	http.HandleFunc("GET /stats", func(w http.ResponseWriter, r *http.Request) {
		stats, err := manager.GetStats()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		body, _ := json.Marshal(stats)
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})

	http.HandleFunc("POST /start", func(w http.ResponseWriter, r *http.Request) {
		type Input struct {
			Tp        string `json:"type"`
			Start     int    `json:"start"`
			End       int    `json:"end"`
			Overwrite bool   `json:"overwrite"`
		}
		var input Input
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			fmt.Println("Error reading body", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		err = json.Unmarshal(bodyBytes, &input)
		if err != nil {
			fmt.Println("Error unmarshalling body", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if input.Tp != "movie" && input.Tp != "show" && input.Tp != "imdb" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Invalid type"))
			return
		}

		if input.Tp == "movie" {
			manager.StartMovieSync(input.Start, input.End, input.Overwrite)
		}

		if input.Tp == "show" {
			manager.StartShowSync(input.Start, input.End, input.Overwrite)
		}

		if input.Tp == "imdb" {
			manager.StartIMDBSync()
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Process started successfully"))
	})

	http.HandleFunc("POST /stop", func(w http.ResponseWriter, r *http.Request) {
		type Input struct {
			Tp        string `json:"type"`
			Start     int    `json:"start"`
			End       int    `json:"end"`
			Overwrite bool   `json:"overwrite"`
		}
		var input Input
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			fmt.Println("Error reading body", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		err = json.Unmarshal(bodyBytes, &input)
		if err != nil {
			fmt.Println("Error unmarshalling body", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if input.Tp != "movie" && input.Tp != "show" && input.Tp != "imdb" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Invalid type"))
			return
		}

		if input.Tp == "movie" {
			manager.StopMovieScrape()
		}

		if input.Tp == "show" {
			manager.StopShowScrape()
		}

		if input.Tp == "imdb" {
			manager.StopImdbScrape()
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Process stopped successfully"))
	})

	go func() {
		fmt.Println("Starting http server")
		err = http.ListenAndServe(":6996", nil)
		if err != nil {
			fmt.Println("Error starting http server", err)
			return
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig

	log.Println("Shutting down...")
	manager.ShutDown()
	db.Close()
}

type Res struct {
	Res *http.Response
	Err error
}

type HttpClient struct {
	timeSinceLast *time.Time
	reqChan       chan *http.Request
	client        *http.Client
	active        map[*http.Request]chan *Res
	mtx           *sync.Mutex
	delay         int
}

func NewClient() *HttpClient {
	var tmdbClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			MaxConnsPerHost:     10,
			IdleConnTimeout:     20 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
	client := &HttpClient{
		reqChan: make(chan *http.Request),
		client:  tmdbClient,
		active:  make(map[*http.Request]chan *Res),
		mtx:     &sync.Mutex{},
		delay:   200,
	}
	go client.Start()
	return client
}

func (c *HttpClient) Do(req *http.Request) (*http.Response, error) {
	cn := c.doInternal(req)
	res := <-cn
	return res.Res, res.Err
}

func (c *HttpClient) doInternal(req *http.Request) chan *Res {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	cn := make(chan *Res)
	c.active[req] = cn

	c.reqChan <- req

	return cn
}

func (c *HttpClient) Start() {
	for v := range c.reqChan {
		v := v
		if c.timeSinceLast != nil {
			nowTime := time.Now()
			sinceLastMilliseconds := nowTime.Sub(*c.timeSinceLast).Milliseconds()
			if sinceLastMilliseconds < int64(c.delay) {
				time.Sleep(time.Duration(int64(c.delay)-sinceLastMilliseconds) * time.Millisecond)
			}
		}
		tm := time.Now()
		c.timeSinceLast = &tm
		go c.sendReq(v)
	}
}

func (c *HttpClient) sendReq(req *http.Request) {
	res, err := c.client.Do(req)
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.active[req] <- &Res{
		Err: err,
		Res: res,
	}
}
