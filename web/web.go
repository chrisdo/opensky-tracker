package web

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/chrisdo/openskytracker"
	"github.com/chrisdo/openskytracker/db"
)

type webServer struct {
	rClient *db.RedisConnector
}

func RunServer(rClient *db.RedisConnector) {
	ws := webServer{rClient}
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)
	http.HandleFunc("/flights", ws.flights)
	http.ListenAndServe(":8081", nil)
}

func (ws *webServer) flights(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()

	//center point
	lat, err := strconv.ParseFloat(params.Get("lat"), 64)
	if err != nil {
		respi, err := json.Marshal(openskytracker.FlightsResponse{Error: "Missing Latitude"})
		if err != nil {
			log.Println(err)
		}
		w.Write(respi)
	}
	lon, err := strconv.ParseFloat(params.Get("lon"), 64)

	if err != nil {
		respi, err := json.Marshal(openskytracker.FlightsResponse{Error: "Missing Longitude"})
		if err != nil {
			log.Println(err)
		}
		w.Write(respi)
	}
	//radius in km
	radius, err := strconv.ParseFloat(params.Get("radius"), 64)

	if err != nil {
		respi, err := json.Marshal(openskytracker.FlightsResponse{Error: "Missing Radius"})
		if err != nil {
			log.Println(err)
		}
		w.Write(respi)
	}

	response := ws.rClient.FetchTracks(lon, lat, radius)

	json, err := json.Marshal(response)
	if err != nil {
		w.Write([]byte(err.Error()))
	} else {
		w.Write([]byte(json))
	}
}
