package main

import (
	"context"
	"database/sql"
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

	go mc.Start()
	go sc.Start()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig

	log.Println("Shutting down...")
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
