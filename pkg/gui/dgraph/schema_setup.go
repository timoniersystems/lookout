package dgraph

import (
	"context"
	"log"
	"time"

	"github.com/dgraph-io/dgo/v210"
	"github.com/dgraph-io/dgo/v210/protos/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func SetupAndRunDgraph() {
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

	defer conn.Close()
	dgraphClient := dgo.NewDgraphClient(api.NewDgraphClient(conn))
	ctx := context.Background()
	setupSchema(dgraphClient, ctx)
}

func setupSchema(client *dgo.Dgraph, ctx context.Context) {
	schema := `
		bomRef: string @index(exact) .
        reference: string @index(exact) .
        name: string @index(exact) .
        version: string @index(exact) .
        purl: string @index(exact) @upsert .
        dependsOn: [uid] @reverse .
		vulnerable: bool @index(bool) .
        cveID: string @index(exact) .
        dgraphURL: string @index(exact) .
        root: bool @index(bool) .

        type Component {
			bomRef
			reference
            name
            version
            purl
            dependsOn
			vulnerable
            cveID
            dgraphURL
            root
        }
    `

	op := &api.Operation{Schema: schema}
	err := client.Alter(ctx, op)
	if err != nil {
		if s, ok := status.FromError(err); ok && (s.Code() == codes.Unavailable || s.Code() == codes.Unknown) {
			log.Printf("Server not ready, unable to alter schema. Error: %v", err)
			time.Sleep(5 * time.Second)
			setupSchema(client, ctx)
		} else {
			log.Fatalf("Failed to alter schema: %v", err)
		}
	} else {
		log.Println("Schema setup successful.")
	}
}
