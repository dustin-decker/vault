package active

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/vault/api"
)

func TestServiceTokens(t *testing.T) {
	type args struct {
	}
	tests := []struct {
		name        string
		clusterSize int
	}{
		{
			name:        "active-active HA cluster",
			clusterSize: ClusterSize,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := GetTestCluster(t, tt.clusterSize)
			if cluster == nil {
				t.Fatal("failed to get test cluster (can it connect to storage?)")
			}
			cluster.Start()

			var secrets = []*api.Secret{}

			// create 1000 child service tokens split among all server clients
			for i := 0; i < 1000; i++ {
				client := cluster.Cores[i%len(cluster.Cores)].Client
				dumbTrueBoolForPointer := true
				secret, err := client.Auth().Token().Create(&api.TokenCreateRequest{
					DisplayName:    fmt.Sprintf("token-%d", i),
					TTL:            "1800",
					ExplicitMaxTTL: "36000",
					Renewable:      &dumbTrueBoolForPointer,
				})
				if err != nil {
					t.Fatal(err)
				}
				if len(secret.Auth.ClientToken) == 0 {
					t.Fatal("got empty client token")
				}
				secrets = append(secrets, secret)
			}

			// lookup all service tokens on all servers to populate caches
			for _, core := range cluster.Cores {
				client := core.Client
				for _, secret := range secrets {
					_, err := client.Auth().Token().Lookup(secret.Auth.ClientToken)
					if err != nil {
						t.Fatal(err)
					}
				}
			}

			// renew all service tokens split among all servers
			for i, secret := range secrets {
				client := cluster.Cores[(i+1)%len(cluster.Cores)].Client
				_, err := client.Auth().Token().Renew(secret.Auth.ClientToken, 20000)
				if err != nil {
					t.Fatal(err)
				}
			}

			// sleep to wait for LRU TTL
			time.Sleep(5)

			// lookup all service tokens on all servers to check renewal
			for _, core := range cluster.Cores {
				client := core.Client
				for _, secret := range secrets {
					secret, err := client.Auth().Token().Lookup(secret.Auth.ClientToken)
					if err != nil {
						t.Fatal(err)
					}
					ttl, err := secret.TokenTTL()
					if err != nil {
						t.Fatal(err)
					}
					if ttl < time.Second*10000 {
						t.Fatal("token does not look renewed from this server")
					}
				}
			}

			// revoke child service tokens split among all server clients
			for i, secret := range secrets {
				client := cluster.Cores[(i+2)%len(cluster.Cores)].Client
				err := client.Auth().Token().RevokeTree(secret.Auth.ClientToken)
				if err != nil {
					t.Fatal(err)
				}
			}

			// sleep to wait for LRU TTL
			time.Sleep(5)

			// lookup revoked service tokens with all servers
			for _, core := range cluster.Cores {
				client := core.Client
				for _, secret := range secrets {
					_, err := client.Auth().Token().Lookup(secret.Auth.ClientToken)
					if err == nil {
						t.Fatal("token should be revoked")
					}
				}
			}
		})
	}
}
