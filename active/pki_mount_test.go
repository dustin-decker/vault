package active

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/hashicorp/vault/api"
)

func TestPKIMount(t *testing.T) {
	type args struct {
	}
	tests := []struct {
		name        string
		clusterSize int
		iterations  int
	}{
		// {
		// 	name:        "single node",
		// 	clusterSize: 1,
		// 	iterations:  10,
		// },
		{
			name:        "active-active HA cluster",
			clusterSize: 3,
			iterations:  30,
		},
	}
	for _, tt := range tests {
		cluster := GetTestCluster(t, tt.clusterSize)
		if cluster == nil {
			t.Fatal("failed to get test cluster (can it connect to storage?)")
		}
		cluster.Start()
		defer cluster.Cleanup()

		t.Run(tt.name, func(t *testing.T) {

			client := cluster.Cores[rand.Intn(len(cluster.Cores))].Client

			// Create root CA mount
			err := client.Sys().Mount("root_ca", &api.MountInput{
				Type: "pki",
				Config: api.MountConfigInput{
					DefaultLeaseTTL: "16h",
					MaxLeaseTTL:     "720h",
				},
			})
			if err != nil {
				t.Fatal(err)
			}

			// Generate the root CA
			_, err = client.Logical().Write("root_ca/root/generate/internal", map[string]interface{}{
				"common_name": "r00t",
				"key_type":    "rsa",
				"key_bits":    "2048",
				"ttl":         "7200h",
			})
			if err != nil {
				t.Fatal(err)
			}

			// Configure signing roles for the root CA
			// Long TTL
			_, err = client.Logical().Write("root_ca/roles/long-ttl", map[string]interface{}{
				"allow_any_name": true,
				"ttl":            86400,
			})
			if err != nil {
				t.Fatal(err)
			}
			// Short TTL
			_, err = client.Logical().Write("root_ca/roles/short-ttl", map[string]interface{}{
				"allow_any_name": true,
				"ttl":            1,
			})
			if err != nil {
				t.Fatal(err)
			}

			// Simulate restart
			for _, core := range cluster.Cores {
				core.LoadMounts(context.Background())
				core.SetupMounts(context.Background())
			}

			time.Sleep(time.Second * 5)

			// Generate short TTL certificate
			var secrets = []*api.Secret{}
			for i := 0; i < tt.iterations; i++ {
				client := cluster.Cores[rand.Intn(len(cluster.Cores))].Client
				secret, err := client.Logical().Write("root_ca/issue/short-ttl", map[string]interface{}{
					"common_name": "short-ttl",
				})
				if err != nil {
					t.Fatal(err)
				}
				secrets = append(secrets, secret)
			}
			// Wait for TTL to expire
			time.Sleep(time.Second * 2)
			// Use Tidy API
			_, err = client.Logical().Write("root_ca/tidy", map[string]interface{}{
				"tidy_cert_store": "true",
				"safety_buffer":   "1s",
			})
			if err != nil {
				t.Fatal(err)
			}
			// Wait for Tidy API
			time.Sleep(time.Second * 1)
			// Expect failure reading short TTL certificate
			for _, core := range cluster.Cores {
				client := core.Client
				for _, secret := range secrets {
					s, err := client.Logical().Read(fmt.Sprintf("root_ca/cert/%s", secret.Data["serial_number"]))
					if err != nil {
						t.Fatal(err)
					}
					if s != nil {
						t.Fatal("short ttl cert should be expired")
					}
				}
			}

			// Generate long TTL certificate
			secrets = []*api.Secret{}
			for i := 0; i < tt.iterations; i++ {
				client := cluster.Cores[rand.Intn(len(cluster.Cores))].Client
				secret, err := client.Logical().Write("root_ca/issue/long-ttl", map[string]interface{}{
					"common_name": "long-ttl",
				})
				if err != nil {
					t.Fatal(err)
				}
				secrets = append(secrets, secret)
			}
			// Expect success reading long TTL certificate
			for _, core := range cluster.Cores {
				client := core.Client
				for _, secret := range secrets {
					_, err = client.Logical().Read(fmt.Sprintf("root_ca/cert/%s", secret.Data["serial_number"]))
					if err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}
