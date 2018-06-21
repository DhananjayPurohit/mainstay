// Http handlers for service requests

package requestapi

import (
    "encoding/json"
    "fmt"
    "net/http"
    "github.com/gorilla/mux"
    "ocean-attestation/models"
)

func Index(w http.ResponseWriter, r *http.Request, channel *models.Channel) {
    fmt.Fprintln(w, "Request Service for Ocean Attestations!")
}

func Block(w http.ResponseWriter, r *http.Request, channel *models.Channel) {
    blockid := mux.Vars(r)["blockId"]
    request := models.Request{Name: mux.CurrentRoute(r).GetName(), Id: blockid,}

    channel.Requests <- request // put request in channel

    response := <- channel.Responses // wait for response from attestation service

    if err := json.NewEncoder(w).Encode(response); err != nil {
        panic(err)
    }
}

func BestBlock(w http.ResponseWriter, r *http.Request, channel *models.Channel) {
    request := models.Request{Name: mux.CurrentRoute(r).GetName(),}

    channel.Requests <- request // put request in channel

    response := <- channel.Responses // wait for response from attestation service

    if err := json.NewEncoder(w).Encode(response); err != nil {
        panic(err)
    }
}

func BestBlockHeight(w http.ResponseWriter, r *http.Request, channel *models.Channel) {
    request := models.Request{Name: mux.CurrentRoute(r).GetName(),}

    channel.Requests <- request // put request in channel

    response := <- channel.Responses // wait for response from attestation service

    if err := json.NewEncoder(w).Encode(response); err != nil {
        panic(err)
    }
}

func Transaction(w http.ResponseWriter, r *http.Request, channel *models.Channel) {
    transactionId := mux.Vars(r)["transactionId"]
    request := models.Request{Name: mux.CurrentRoute(r).GetName(), Id: transactionId,}

    channel.Requests <- request // put request in channel

    response := <- channel.Responses // wait for response from attestation service

    if err := json.NewEncoder(w).Encode(response); err != nil {
        panic(err)
    }
}

func LatestAttestation(w http.ResponseWriter, r *http.Request, channel *models.Channel) {
    request := models.Request{Name: mux.CurrentRoute(r).GetName(),}

    channel.Requests <- request // put request in channel

    response := <- channel.Responses // wait for response from attestation service

    if err := json.NewEncoder(w).Encode(response); err != nil {
        panic(err)
    }
}
