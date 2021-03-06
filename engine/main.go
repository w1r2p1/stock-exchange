package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/briandowns/stock-exchange/config"
	"github.com/briandowns/stock-exchange/database"
	"github.com/briandowns/stock-exchange/models"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"github.com/thoas/stats"
	"github.com/unrolled/render"
)

var cache Cacher
var signalsChan = make(chan os.Signal, 1)

func main() {
	signal.Notify(signalsChan, os.Interrupt)

	go func() {
		for sig := range signalsChan {
			fmt.Printf("\nEngine shutting down... %v\n", sig)
			signalsChan = nil
			os.Exit(1)
		}
	}()

	// load configuration
	config, err := config.Load("../config.json")
	if err != nil {
		log.Fatal(err)
	}

	// TODO(briandowns) this needs to be simplified 2016-06-22T16:09 4
	switch config.Engine.CacheLocation {
	case "redis":
		log.Println("Using Redis...")
		cache = Cache{NewRedisCache(config)}
	case "boltdb":
		log.Println("Using BoltDB...")
		db, err := database.NewDB(config.Cache.BoltDB.Name)
		if err != nil {
			log.Fatal(err)
		}
		cache = Cache{NewBoltCache(db, config)}
	default:
		log.Fatal(errUnknownCache)
	}
	if err := cache.Build(); err != nil {
		log.Fatal(err)
	}

	ob := &models.OrderBook{models.NewNasdaqOrderBook()}

	// initialize the API
	n := negroni.New(
		negroni.NewRecovery(),
		negroni.NewLogger(),
	)
	statsMiddleware := stats.New()
	ren := render.New()

	// create the router
	router := mux.NewRouter()

	// route handler for a health check
	//router.HandleFunc(HealthCheckPath, HealthCheckHandler()).Methods("HEAD")
	router.HandleFunc(HealthCheckPath, HealthCheckHandler).Methods("HEAD")

	// route handler for statistics
	router.HandleFunc(StatsPath, StatsHandler(ren, statsMiddleware)).Methods("GET")

	// route handler for the book
	router.HandleFunc(BookPath, BookHandler(ren, ob)).Methods("GET")

	// route handler for individual book entries
	router.HandleFunc(BookEntryByIDPath, BookEntryByIDHandler(ren)).Methods("GET")

	// route handler for viewing symbol data
	router.HandleFunc(SymbolsPath, SymbolsHandler(ren, cache)).Methods("GET")

	// route handler for viewing symbol data by ID
	router.HandleFunc(SymbolByIDPath, SymbolByIDHandler(ren, cache)).Methods("GET")

	// route handler for adding trades
	router.HandleFunc(OrderPath, AddOrderHandler(ren, ob)).Methods("POST")

	// route handler for canceling trades
	router.HandleFunc(CancelOrderPath, CancelTradeHandler(ren, ob)).Methods("DELETE")

	n.Use(statsMiddleware)
	n.UseHandler(router)
	n.Run(":7777")
}
