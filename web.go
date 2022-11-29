package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-redis/redis/v8"
)

type FlightsResponse struct {
	Error   string   `json:"error"`
	Flights []Flight `json:"flights"`
}

type Flight struct {
	ID       string  `json:"id,omitempty"`
	Lat      float64 `json:"lat,omitempty"`
	Lon      float64 `json:"lon,omitempty"`
	Heading  float64 `json:"heading,omitempty"`
	Altitude float64 `json:"altitude,omitempty"`
	Callsign string  `json:"callsign,omitempty"`
}

func flights(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()

	//center point
	lat, err := strconv.ParseFloat(params.Get("lat"), 64)
	if err != nil {
		respi, err := json.Marshal(FlightsResponse{Error: "Missing Latitude"})
		if err != nil {
			log.Println(err)
		}
		w.Write(respi)
	}
	lon, err := strconv.ParseFloat(params.Get("lon"), 64)

	if err != nil {
		respi, err := json.Marshal(FlightsResponse{Error: "Missing Longitude"})
		if err != nil {
			log.Println(err)
		}
		w.Write(respi)
	}
	//radius in km
	radius, err := strconv.ParseFloat(params.Get("radius"), 64)

	if err != nil {
		respi, err := json.Marshal(FlightsResponse{Error: "Missing Radius"})
		if err != nil {
			log.Println(err)
		}
		w.Write(respi)
	}
	query := &redis.GeoRadiusQuery{
		Radius:    radius,
		Unit:      "km",
		WithCoord: true,
	}

	//we get the bounding box, but actually we need the center point and width and height

	flights, err := redisClient.GeoRadius(redisClient.Context(), KEY_FLIGHT_POSITIONS, lon, lat, query).Result()

	if err != nil {
		w.Write([]byte(err.Error()))
	}

	response := FlightsResponse{Flights: make([]Flight, 0)}

	for _, f := range flights {

		flight := Flight{ID: f.Name, Lat: f.Latitude, Lon: f.Longitude}

		callsign, err := redisClient.HGet(redisClient.Context(), KEY_TRACK_PREFIX+f.Name, "Callsign").Result()
		if err != nil {

			log.Println(err)
		}
		flight.Callsign = callsign
		hdg, err := redisClient.HGet(redisClient.Context(), KEY_TRACK_PREFIX+f.Name, "Heading").Result()
		flight.Heading, _ = strconv.ParseFloat(hdg, 64)
		response.Flights = append(response.Flights, flight)
	}
	json, err := json.Marshal(response)
	if err != nil {
		w.Write([]byte(err.Error()))
	} else {
		w.Write([]byte(json))
	}
}
