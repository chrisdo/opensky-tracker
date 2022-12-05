package db

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/chrisdo/opensky-go-api"
	"github.com/chrisdo/openskytracker"
	"github.com/go-redis/redis/v8"
)

const (
	KEY_FLIGHT_POSITIONS     string = "flights:positions"
	KEY_TRACK_PREFIX         string = "tracks:"
	KEY_FLIGHTS_LAST_CONTACT string = "flights:LastContact"
)

type RedisConnector struct {
	client *redis.Client
}

type TrackFetcher interface {
	FetchTracks(lon, lat, radius float64) openskytracker.FlightsResponse
}

func NewRedisConnector(host, port string) *RedisConnector {
	redisClient := redis.NewClient(&redis.Options{Addr: fmt.Sprintf("%s:%s", host, port), Password: "", DB: 0})

	_, err := redisClient.Ping(redisClient.Context()).Result()
	if err != nil {
		panic(err)
	}
	log.Println("Succesfully connected to REDIS ")

	return &RedisConnector{redisClient}
}

func (rc *RedisConnector) Run() {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	client := opensky.NewClient()

	stateVectorRequest := opensky.NewStateVectorRequest().IncludeCategory()

	//we also want to run some cleanup task, that checks regularly the flights from zset with Score older than now-x minutes -> remove, or archive from zset/hset /geolist
	//we want to fire immediately, and then we wait for the timer
	go func() {
		for {
			rc.fetchAndUpdateStateVectors(client, stateVectorRequest)
			rc.cleanup() //just for simplicity now. ideally we want to separte the cleanup timer from fetching data.
			<-ticker.C
		}
	}()
}

func (rc *RedisConnector) Close() {
	rc.client.Close()
}

func (rc *RedisConnector) FetchTracks(lon float64, lat float64, radius float64) openskytracker.FlightsResponse {
	query := &redis.GeoRadiusQuery{
		Radius:    radius,
		Unit:      "km",
		WithCoord: true,
	}

	//we get the bounding box, but actually we need the center point and width and height

	flights, err := rc.client.GeoRadius(rc.client.Context(), KEY_FLIGHT_POSITIONS, lon, lat, query).Result()

	if err != nil {
		return openskytracker.FlightsResponse{Error: err.Error()}
	}

	response := openskytracker.FlightsResponse{Flights: make([]openskytracker.Flight, 0)}

	for _, f := range flights {

		flight := openskytracker.Flight{ID: f.Name, Lat: f.Latitude, Lon: f.Longitude}

		callsign, err := rc.client.HGet(rc.client.Context(), KEY_TRACK_PREFIX+f.Name, "Callsign").Result()
		if err != nil {

			log.Println(err)
		}
		flight.Callsign = callsign
		hdg, _ := rc.client.HGet(rc.client.Context(), KEY_TRACK_PREFIX+f.Name, "Heading").Result()
		flight.Heading, _ = strconv.ParseFloat(hdg, 64)
		response.Flights = append(response.Flights, flight)
	}
	return response
}

func (rc *RedisConnector) fetchAndUpdateStateVectors(client *opensky.Client, stateVectorRequest *opensky.StateVectorRequest) {

	vectors, err := client.RequestStateVectors(stateVectorRequest)
	if err != nil {
		log.Println(err)
		return
	}
	log.Printf("Received #StateVectors: %d\n", len(vectors.States))
	pipe := rc.client.Pipeline()
	ctx := rc.client.Context()
	geoadds := make([]*redis.GeoLocation, 0)
	for _, v := range vectors.States {
		if v.Latitude.Valid && v.Longitude.Valid {
			geoadds = append(geoadds, createGeoAddCommand(&v))
			setLastUpdated(ctx, &v, pipe)
			writeStateVectorHashToRedis(ctx, &v, pipe)
		}
	}
	pipe.GeoAdd(ctx, KEY_FLIGHT_POSITIONS, geoadds...)
	_, err = pipe.Exec(ctx)

	if err != nil {
		log.Println("Error while executing pipe: ", err)
	}
	log.Printf("Added positions for #flights: %d\n", len(geoadds))
}

func (rc *RedisConnector) cleanup() {
	t := time.Now().UTC()

	ctx := rc.client.Context()
	keys, err := rc.client.ZRangeByScore(ctx, KEY_FLIGHTS_LAST_CONTACT, &redis.ZRangeBy{Min: "-inf", Max: strconv.Itoa(int(t.Add(-5 * time.Minute).UnixMilli()))}).Result()

	if err != nil {
		log.Println("Error while getting keys to remove: ", err)
		return
	}
	log.Printf("Removing keys: %s\n", keys)
	removed, _ := rc.client.ZRem(ctx, KEY_FLIGHTS_LAST_CONTACT, keys).Result()
	log.Printf("Removed from flights:LastContact : %d\n", removed)
	removed, _ = rc.client.ZRem(ctx, KEY_FLIGHT_POSITIONS, keys).Result()

	log.Printf("Removed from flights:positions : %d\n", removed)
	trackKeys := make([]string, removed)
	for i, k := range keys {
		trackKeys[i] = KEY_TRACK_PREFIX + k
	}
	removed, _ = rc.client.Del(ctx, trackKeys...).Result()
	log.Printf("Removed from tracks: %d\n", removed)
}

func createGeoAddCommand(v *opensky.StateVector) *redis.GeoLocation {
	return &redis.GeoLocation{
		Name:      v.Icao24,
		Longitude: v.Longitude.Value,
		Latitude:  v.Latitude.Value,
	}
}

func setLastUpdated(ctx context.Context, v *opensky.StateVector, pipe redis.Pipeliner) {
	//we could also use EXPIRE on the tracks:<modesKey>. but ok, for now lets just use this pattern for cleanups
	pipe.ZAdd(ctx, KEY_FLIGHTS_LAST_CONTACT, &redis.Z{Score: float64(v.LastContact.UnixMilli()), Member: v.Icao24})
}

func writeStateVectorHashToRedis(ctx context.Context, v *opensky.StateVector, pipe redis.Pipeliner) {

	pipe.HSet(ctx, KEY_TRACK_PREFIX+v.Icao24, "Callsign", v.Callsign, "Squawk", v.Squawk, "Spi", v.Spi, "OnGround", v.OnGround, "Heading", v.TrueTrack.Value, "Category", v.Category.String(), "Source", v.PositionSource.String())

}
