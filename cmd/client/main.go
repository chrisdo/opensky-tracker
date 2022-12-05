package main

import (
	"context"
	"io"
	"log"
	"os"
	"strconv"

	"github.com/chrisdo/openskytracker/notificator"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var tracks map[string]*notificator.FlightStatus

func main() {

	args := os.Args[1:]
	if len(args) != 3 {
		panic("Must provide 3 arguments: lat lon radius")
	}
	lat, err := strconv.ParseFloat(args[0], 64)
	if err != nil {
		panic(err)
	}
	lon, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		panic(err)
	}
	radius, err := strconv.Atoi(args[2])
	if err != nil {
		panic(err)
	}

	var opts []grpc.DialOption

	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	conn, err := grpc.Dial("172.18.0.1:50032", opts...)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	tracks = make(map[string]*notificator.FlightStatus)

	client := notificator.NewNotificatorClient(conn)
	log.Printf("Requesting Updates for Lat:%f, Lon:%f, Radius: %d\n", lat, lon, radius)
	updats, err := client.GetRegionUpdates(context.Background(), &notificator.Region{Center: &notificator.Location{Latitude: lat, Longitude: lon}, Radius: int32(radius)})

	if err != nil {
		panic(err)
	}
	log.Println("Waiting for updates")
	for {
		t, err := updats.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("client.GetRegionUpdates failed: %v", err)
		}
		if _, ok := tracks[t.ModesId]; !ok {
			log.Printf("##### NEW AIRCAFT ENTERED REGION: %s ####", t.ModesId)
			//now we should write this to USB SERIAL PORT and show it on the arduino display
			tracks[t.ModesId] = t
		}
		//from time to time clean up this map
		//log.Printf("%v\n", t)
		//if its a new one, give me a signal
	}
	log.Println("Stopping client")
}
