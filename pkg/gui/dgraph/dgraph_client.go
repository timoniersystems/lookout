package dgraph

import (
	"context"
	"log"
	"time"

	"github.com/dgraph-io/dgo/v210"
	"github.com/dgraph-io/dgo/v210/protos/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

func DgraphClient() *dgo.Dgraph {
	var conn *grpc.ClientConn
	var err error

	for i := 0; i < 5; i++ {
		conn, err = grpc.Dial("alpha:9080", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
		if err == nil && conn.GetState() == connectivity.Ready {
			break
		}
		if i < 4 {
			log.Printf("Retrying to connect: attempt %d", i+1)
			time.Sleep(2 * time.Second)
		}
	}

	if err != nil {
		log.Fatalf("While trying to dial gRPC after retries: %v", err)
	}

	dgraphClient := dgo.NewDgraphClient(api.NewDgraphClient(conn))

	return dgraphClient
}

func DropAllData(client *dgo.Dgraph) error {
	ctx := context.Background()
	op := &api.Operation{DropOp: api.Operation_DATA}

	if err := client.Alter(ctx, op); err != nil {
		log.Printf("Failed to drop all data: %v", err)
		return err
	}

	log.Println("Successfully dropped all data (schema preserved).")
	return nil
}
