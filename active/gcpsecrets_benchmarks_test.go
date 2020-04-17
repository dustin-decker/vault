package active

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/vault/api"
)

func BenchmarkGCPSecrets(b *testing.B) {
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

		// Create GCP backend mount
		// Uses GOOGLE_APPLICATION_CREDENTIALS by default
		err := client.Sys().Mount("gcp", &api.MountInput{
			Type: "gcp",
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

		bindingTemplate := `
resource "//cloudresourcemanager.googleapis.com/projects/%s" {
	roles = [
		"roles/storage.admin",
	]
}
`
		binding := strings.TrimSpace(fmt.Sprintf(bindingTemplate, GCPProject))

		// Create short lived roleset
		client = cluster.Cores[rand.Intn(len(cluster.Cores))].Client
		_, err = client.Logical().Write("gcp/roleset/test-ttl", map[string]interface{}{
			"secret_type":  "access_token",
			"token_scopes": []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/cloud-platform"},
			"project":      GCPProject,
			"bindings":     binding,
		})
		if err != nil {
			b.Fatal(err)
		}

		// Wait for roleset to be created
		time.Sleep(time.Second * 5)

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				client = cluster.Cores[rand.Intn(len(cluster.Cores))].Client

				// Obtain credential from roleset
				_, err := client.Logical().Read("gcp/token/test-ttl")
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
