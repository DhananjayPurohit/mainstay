// Routing for http requests to request service
package main

import (
    "net/http"
    "time"
    "log"
    "github.com/gorilla/mux"
)

type Route struct {
    name        string
    method      string
    pattern     string
    handlerFunc func(http.ResponseWriter, *http.Request, chan Request)
}

var routes = []Route{
    Route{
        "Index",
        "GET",
        "/",
        Index,
    },
    Route{
        "Block",
        "GET",
        "/block/{blockId}",
        Block,
    },
    Route{
        "BestBlock",
        "GET",
        "/bestblock/",
        BestBlock,
    },
}

func NewRouter(reqs chan Request) *mux.Router {
    router := mux.NewRouter().StrictSlash(true)
    for _, route := range routes {
        handlerFunc := makeHandler(route.handlerFunc, reqs) // pass channel to request handler
        router.
            Methods(route.method).
            Path(route.pattern).
            Name(route.name).
            Handler(handlerFunc)
    }
    return router
}

func makeHandler(fn func (http.ResponseWriter, *http.Request, chan Request), reqs chan Request) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        fn(w, r, reqs)
        log.Printf("%s\t%s\t%s", r.Method, r.RequestURI, time.Since(start),)
    }
}