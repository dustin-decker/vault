package active

import (
	"fmt"
	"math/rand"
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
			tokens:      300,
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

			// create child service tokens split among all server clients
			for i := 0; i < tt.tokens; i++ {
				client := cluster.Cores[rand.Intn(len(cluster.Cores))].Client
				dumbTrueBoolForPointer := true
				secret, err := client.Auth().Token().Create(&api.TokenCreateRequest{
					DisplayName:    fmt.Sprintf("token-%d", i),
					TTL:            "1800s",
					ExplicitMaxTTL: "36000s",
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
			for _, secret := range secrets {
				client := cluster.Cores[rand.Intn(len(cluster.Cores))].Client
				_, err := client.Auth().Token().Renew(secret.Auth.ClientToken, 20000)
				if err != nil {
					t.Fatal(err)
				}
			}

			// sleep to wait for TTL
			time.Sleep(time.Millisecond * 1)

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
					if ttl < time.Second*1800 {
						t.Fatalf("token does not look renewed from this server, got %s", ttl)
					}
				}
			}

			// revoke child service tokens split among all server clients
			for _, secret := range secrets {
				client := cluster.Cores[rand.Intn(len(cluster.Cores))].Client
				err := client.Auth().Token().RevokeTree(secret.Auth.ClientToken)
				if err != nil {
					t.Fatal(err)
				}
			}

			// sleep to wait for TTL
			time.Sleep(time.Millisecond)

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

			// create short lived service tokens
			secrets = []*api.Secret{}
			for i := 0; i < tt.tokens; i++ {
				client := cluster.Cores[rand.Intn(len(cluster.Cores))].Client
				secret, err := client.Auth().Token().Create(&api.TokenCreateRequest{
					DisplayName: "token",
					TTL:         "1s",
				})
				if err != nil {
					t.Fatal(err)
				}
				secrets = append(secrets, secret)
			}
			// wait for tokens to expire
			time.Sleep(time.Millisecond * 1100)
			for _, secret := range secrets {
				// expect token to be expired
				client := cluster.Cores[rand.Intn(len(cluster.Cores))].Client
				_, err := client.Auth().Token().Lookup(secret.Auth.ClientToken)
				if err == nil {
					t.Fatal("expected token lookup on expired service token to fail")
				}
			}
		})
	}
}
