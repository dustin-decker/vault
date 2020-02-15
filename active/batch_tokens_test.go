package active

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/hashicorp/vault/api"
)

func TestBatchTokens(t *testing.T) {
	type args struct {
	}
	tests := []struct {
		name        string
		clusterSize int
		tokens      int
	}{
		{
			name:        "single node",
			clusterSize: 1,
			tokens:      10,
		},
		{
			name:        "active-active HA cluster",
			clusterSize: 3,
			tokens:      100,
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

			// create batch tokens split among all server clients
			for i := 0; i < tt.tokens; i++ {
				client := cluster.Cores[rand.Intn(len(cluster.Cores))].Client
				secret, err := client.Auth().Token().Create(&api.TokenCreateRequest{
					DisplayName: fmt.Sprintf("token-%d", i),
					TTL:         "1800s",
					Policies:    []string{"some-policy"},
					Type:        "batch",
				})
				if err != nil {
					t.Fatal(err)
				}
				if len(secret.Auth.ClientToken) == 0 {
					t.Fatal("got empty client token")
				}
				secrets = append(secrets, secret)
			}

			// lookup all batch tokens on all servers
			for _, core := range cluster.Cores {
				client := core.Client
				for _, secret := range secrets {
					s, err := client.Auth().Token().Lookup(secret.Auth.ClientToken)
					if err != nil {
						t.Fatal(err)
					}
					if len(s.Data["policies"].([]interface{})) != 2 {
						t.Fatal("didn't get expected policy")
					}
				}
			}

			// create short lived batch token
			secrets = []*api.Secret{}
			for i := 0; i < tt.tokens; i++ {
				client := cluster.Cores[rand.Intn(len(cluster.Cores))].Client
				secret, err := client.Auth().Token().Create(&api.TokenCreateRequest{
					DisplayName: "token",
					TTL:         "1s",
					Policies:    []string{"some-policy"},
					Type:        "batch",
				})
				if err != nil {
					t.Fatal(err)
				}
				secrets = append(secrets, secret)
			}
			// sleep to wait for expiration
			time.Sleep(time.Millisecond * 1100)
			for _, secret := range secrets {
				// expect token to be expired
				client := cluster.Cores[rand.Intn(len(cluster.Cores))].Client
				_, err := client.Auth().Token().Lookup(secret.Auth.ClientToken)
				if err == nil {
					t.Fatal("expected token lookup on expired batch token to fail")
				}
			}
		})
	}
}
