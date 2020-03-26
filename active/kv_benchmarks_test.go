package active

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/vault/api"
)

func BenchmarkKV(b *testing.B) {
	type args struct {
	}
	tests := []struct {
		name        string
		clusterSize int
	}{
		{
			name:        "single node",
			clusterSize: 1,
		},
		{
			name:        "active-active HA cluster",
			clusterSize: 3,
		},
	}
	for _, tt := range tests {
		cluster := GetTestCluster(b, tt.clusterSize)
		if cluster == nil {
			b.Fatal("failed to get test cluster (can it connect to storage?)")
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
			b.Fatal(err)
		}

		// Simulate restart after enabling mount
		for _, core := range cluster.Cores {
			core.LoadMounts(context.Background())
			core.SetupMounts(context.Background())
		}

		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				client = cluster.Cores[rand.Intn(len(cluster.Cores))].Client

				_, err := client.Logical().Write(fmt.Sprintf("kv/secret-%d", i),
					map[string]interface{}{
						"value": fmt.Sprintf("%d", i),
					})
				if err != nil {
					b.Fatal(err)
				}
				i++
			}
		})
	}
}
