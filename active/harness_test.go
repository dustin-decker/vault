package active

import (
	"testing"

	"github.com/hashicorp/vault/api"
)

func TestGetTestCluster(t *testing.T) {
	type args struct {
	}
	tests := []struct {
		name    string
		servers int
	}{
		{
			name:    "get three mysql backed servers",
			servers: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := getTestCluster(t, tt.servers)
			cluster.Start()

			for _, core := range cluster.Cores {
				client := core.Client
				_, err := client.Auth().Token().LookupSelf()
				if err != nil {
					t.Fatal(err)
				}

				status, err := client.Sys().SealStatus()
				if err != nil {
					t.Fatal(err)
				}
				if status.Sealed {
					t.Fatal("should not be sealed")
				}

				secret, err := client.Auth().Token().Create(&api.TokenCreateRequest{DisplayName: "lol", TTL: "5"})
				if err != nil {
					t.Fatal(err)
				}
				if len(secret.Auth.ClientToken) == 0 {
					t.Fatal("got empty client token")
				}
			}

		})
	}
}
