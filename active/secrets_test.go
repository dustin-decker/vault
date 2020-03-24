package active

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/vault/api"
)

func TestSecrets(t *testing.T) {
	type args struct {
	}
	tests := []struct {
		name        string
		clusterSize int
		iterations  int
	}{
		{
			name:        "single node",
			clusterSize: 1,
			iterations:  10,
		},
		{
			name:        "active-active HA cluster",
			clusterSize: 3,
			iterations:  100,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := GetTestCluster(t, tt.clusterSize)
			if cluster == nil {
				t.Fatal("failed to get test cluster (can it connect to storage?)")
			}
			cluster.Start()
			defer cluster.Cleanup()

			client := cluster.Cores[rand.Intn(len(cluster.Cores))].Client

			// Create secrets backend mount
			err := client.Sys().Mount("kv", &api.MountInput{
				Type: "kv",
				Config: api.MountConfigInput{
					DefaultLeaseTTL: "10s",
					MaxLeaseTTL:     "24h",
				},
			})
			if err != nil {
				t.Fatal(err)
			}

			// Simulate restart after enabling mount
			for _, core := range cluster.Cores {
				core.LoadMounts(context.Background())
				core.SetupMounts(context.Background())
			}

			// Populate secrets
			for i := 0; i < tt.iterations; i++ {
				client = cluster.Cores[rand.Intn(len(cluster.Cores))].Client

				_, err := client.Logical().Write(fmt.Sprintf("kv/secret-%d", i),
					map[string]interface{}{
						"value": fmt.Sprintf("%d", i),
					})
				if err != nil {
					t.Fatal(err)
				}
			}

			// Wait for cache ttl expiration
			time.Sleep(time.Second * 1)

			// Check for expected secrets
			for i := 0; i < tt.iterations; i++ {
				client = cluster.Cores[rand.Intn(len(cluster.Cores))].Client

				s, err := client.Logical().Read(fmt.Sprintf("kv/secret-%d", i))
				if err != nil {
					t.Fatal(err)
				}

				if !reflect.DeepEqual(s.Data, map[string]interface{}{"value": fmt.Sprintf("%d", i)}) {
					t.Fatal("didn't get expected secret")
				}
			}
		})
	}
}
