package active

import (
	"testing"

	"github.com/hashicorp/vault/api"
)

func TestGetTestCluster(t *testing.T) {
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
		cluster := GetTestCluster(t, tt.clusterSize)
		if cluster == nil {
			t.Fatal("failed to get test cluster (can it connect to storage?)")
		}
		cluster.Start()
		defer cluster.Cleanup()

		t.Run(tt.name, func(t *testing.T) {

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
