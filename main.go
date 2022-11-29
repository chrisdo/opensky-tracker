/*
This is a tool for demonstation purpose. Since we do not have an opensky account, we ca only do ratelimited requests: 100 per day,
so roughly 4 per hour.
Additionally, the time resolution is 10 seconds, so we will get the data from t:= now(), t-(t%10). But this is not important here :)
we will create a timer so we run 4 times per hour, fetch all data and update what we have in redis
*/
package main

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/chrisdo/opensky-go-api"
	"github.com/go-redis/redis/v8"
)

var client *opensky.Client
var stateVectorRequest *opensky.StateVectorRequest
var redisClient *redis.Client

const (
	KEY_FLIGHT_POSITIONS     string = "flights:positions"
	KEY_TRACK_PREFIX         string = "tracks:"
	KEY_FLIGHTS_LAST_CONTACT string = "flights:LastContact"
)

func main() {

	redisClient = redis.NewClient(&redis.Options{Addr: "localhost:6379", Password: "", DB: 0})
	defer redisClient.Close()

	_, err := redisClient.Ping(redisClient.Context()).Result()
	if err != nil {
		panic(err)
	}
	log.Println("Succesfully connected to REDIS ")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	client = opensky.NewClient()

	stateVectorRequest = opensky.NewStateVectorRequest().IncludeCategory()

	//we also want to run some cleanup task, that checks regularly the flights from zset with Score older than now-x minutes -> remove, or archive from zset/hset /geolist
	//we want to fire immediately, and then we wait for the timer
	go func() {
		for {
			fetchAndUpdateStateVectors()
			cleanup() //just for simplicity now. ideally we want to separte the cleanup timer from fetching data. when we do this, we could do the cleanup in the same pipe afterwards..
			<-ticker.C
		}
	}()

	//lets create a http server so we can get some status via web client
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)
	http.HandleFunc("/flights", flights)
	http.ListenAndServe("127.0.0.1:8081", nil)

}

func fetchAndUpdateStateVectors() {

	vectors, err := client.RequestStateVectors(stateVectorRequest)
	if err != nil {
		log.Println(err)
		return
	}
	log.Printf("Received #StateVectors: %d\n", len(vectors.States))
	pipe := redisClient.Pipeline()
	ctx := redisClient.Context()
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

func cleanup() {
	t := time.Now().UTC()

	ctx := redisClient.Context()
	keys, err := redisClient.ZRangeByScore(ctx, KEY_FLIGHTS_LAST_CONTACT, &redis.ZRangeBy{Min: "-inf", Max: strconv.Itoa(int(t.Add(-5 * time.Minute).UnixMilli()))}).Result()

	if err != nil {
		log.Println("Error while getting keys to remove: ", err)
		return
	}
	log.Printf("Removing keys: %s\n", keys)
	removed, err := redisClient.ZRem(ctx, KEY_FLIGHTS_LAST_CONTACT, keys).Result()
	log.Printf("Removed from flights:LastContact : %d\n", removed)
	removed, err = redisClient.ZRem(ctx, KEY_FLIGHT_POSITIONS, keys).Result()

	log.Printf("Removed from flights:positions : %d\n", removed)
	trackKeys := make([]string, removed)
	for i, k := range keys {
		trackKeys[i] = KEY_TRACK_PREFIX + k
	}
	removed, err = redisClient.Del(ctx, trackKeys...).Result()
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
