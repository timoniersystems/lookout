package dgraph

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/timoniersystems/lookout/pkg/config"
	"github.com/timoniersystems/lookout/pkg/logging"

	"github.com/dgraph-io/dgo/v210"
	"github.com/dgraph-io/dgo/v210/protos/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

// DgraphClientManager manages a singleton Dgraph client with proper connection lifecycle
type DgraphClientManager struct {
	client     *dgo.Dgraph
	conn       *grpc.ClientConn
	mu         sync.Mutex
	host       string
	port       string
	maxRetries int
	retryDelay time.Duration
}

var (
	globalClientManager *DgraphClientManager
	once                sync.Once
)

// GetGlobalClientManager returns the singleton Dgraph client manager
func GetGlobalClientManager() *DgraphClientManager {
	once.Do(func() {
		// Load configuration from environment variables
		cfg, err := config.Load()
		if err != nil {
			logging.Warn("Failed to load config, using defaults: %v", err)
			cfg = &config.Config{}
			cfg.Dgraph.Host = "alpha"
			cfg.Dgraph.Port = 9080
			cfg.Dgraph.MaxRetries = 5
			cfg.Dgraph.RetryDelayMS = 2000
		}

		globalClientManager = &DgraphClientManager{
			host:       cfg.Dgraph.Host,
			port:       fmt.Sprintf("%d", cfg.Dgraph.Port),
			maxRetries: cfg.Dgraph.MaxRetries,
			retryDelay: time.Duration(cfg.Dgraph.RetryDelayMS) * time.Millisecond,
		}
	})
	return globalClientManager
}

// NewDgraphClientManager creates a new Dgraph client manager with custom configuration
func NewDgraphClientManager(host, port string, maxRetries int, retryDelay time.Duration) *DgraphClientManager {
	return &DgraphClientManager{
		host:       host,
		port:       port,
		maxRetries: maxRetries,
		retryDelay: retryDelay,
	}
}

// Connect establishes a connection to Dgraph with retry logic
func (m *DgraphClientManager) Connect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.client != nil {
		return nil // Already connected
	}

	var conn *grpc.ClientConn
	var err error

	address := fmt.Sprintf("%s:%s", m.host, m.port)

	for i := 0; i < m.maxRetries; i++ {
		conn, err = grpc.Dial(address,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(10*1024*1024)), // 10MB
		)
		if err == nil && conn.GetState() == connectivity.Ready {
			break
		}
		if i < m.maxRetries-1 {
			logging.Info("Retrying to connect to Dgraph at %s: attempt %d", address, i+1)
			time.Sleep(m.retryDelay)
		}
	}

	if err != nil {
		return fmt.Errorf("failed to dial Dgraph after retries: %w", err)
	}

	m.conn = conn
	m.client = dgo.NewDgraphClient(api.NewDgraphClient(conn))

	logging.Info("Successfully connected to Dgraph at %s", address)
	return nil
}

// GetClient returns the Dgraph client, connecting if necessary
func (m *DgraphClientManager) GetClient() (*dgo.Dgraph, error) {
	if m.client == nil {
		if err := m.Connect(); err != nil {
			return nil, err
		}
	}
	return m.client, nil
}

// Close closes the Dgraph connection
func (m *DgraphClientManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.conn != nil {
		err := m.conn.Close()
		m.conn = nil
		m.client = nil
		return err
	}
	return nil
}

// DgraphClient returns the global Dgraph client (deprecated - use GetGlobalClientManager().GetClient())
// This function is kept for backward compatibility but should be migrated away from
func DgraphClient() *dgo.Dgraph {
	manager := GetGlobalClientManager()
	client, err := manager.GetClient()
	if err != nil {
		logging.Error("Failed to get Dgraph client: %v", err)
		panic(fmt.Sprintf("Failed to get Dgraph client: %v", err))
	}
	return client
}

func DropAllData(client *dgo.Dgraph) error {
	ctx := context.Background()
	op := &api.Operation{DropOp: api.Operation_ALL}

	if err := client.Alter(ctx, op); err != nil {
		logging.Error("Failed to drop all data: %v", err)
		return err
	}

	logging.Info("Successfully dropped all data and schema")
	return nil
}
