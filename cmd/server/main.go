/*
This is a tool for demonstation purpose. Since we do not have an opensky account, we ca only do ratelimited requests: 100 per day,
so roughly 4 per hour.
Additionally, the time resolution is 10 seconds, so we will get the data from t:= now(), t-(t%10). But this is not important here :)
we will create a timer so we run 4 times per hour, fetch all data and update what we have in redis
*/
package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/chrisdo/openskytracker/db"
	"github.com/chrisdo/openskytracker/notificator"
	"github.com/chrisdo/openskytracker/web"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

type notificatorServer struct {
	notificator.UnimplementedNotificatorServer
	rClient *db.RedisConnector
}

func main() {
	redisHost, set := os.LookupEnv("REDISHOST")
	if !set {
		redisHost = "localhost"
	}
	redisPort, set := os.LookupEnv("REDISPORT")
	if !set {
		redisPort = "6379"
	}
	grpcHost, set := os.LookupEnv("GRPCHOST")
	if !set {
		grpcHost = ""
	}
	grpcServerPort, set := os.LookupEnv("GRPCSERVERPORT")
	if !set {
		grpcServerPort = "50032"
	}

	//init redis connector. If it does not succeed, it will panic
	redisConnector := db.NewRedisConnector(redisHost, redisPort)
	defer redisConnector.Close()
	redisConnector.Run()

	//init grpc server
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%s", grpcHost, grpcServerPort))
	defer lis.Close()
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcRegionUpdateServer := &notificatorServer{rClient: redisConnector}

	var opts []grpc.ServerOption
	//weshouldinitlaise TLS options here, but lets leave that for now
	grpcServer := grpc.NewServer(opts...)
	notificator.RegisterNotificatorServer(grpcServer, grpcRegionUpdateServer)
	log.Println("Listening on localhost:50032")
	go grpcServer.Serve(lis)

	//init and tun webserver. This will block
	web.RunServer(redisConnector)

}

func (ns *notificatorServer) GetRegionUpdates(r *notificator.Region, s notificator.Notificator_GetRegionUpdatesServer) error {
	//get the region from client, create keep those regions
	peer, _ := peer.FromContext(s.Context())
	log.Printf("Connection from %s\n", peer.Addr.String())
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	var err error = nil
	for err == nil {
		tracks := ns.rClient.FetchTracks(r.Center.Longitude, r.Center.Latitude, float64(r.Radius))
		for _, t := range tracks.Flights {
			err = s.Send(&notificator.FlightStatus{Callsign: t.Callsign, ModesId: t.ID, Position: &notificator.Location{Latitude: t.Lat, Longitude: t.Lon}})
			if err != nil {
				log.Printf("Client %s closed: %s\n", peer.Addr.String(), err)
				break
			}
		}
		<-ticker.C
	}
	return nil
}

func (ns *notificatorServer) mustEmbedUnimplementedNotificatorServer() {
	panic("not implemented")
}
