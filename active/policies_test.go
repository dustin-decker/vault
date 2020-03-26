package active

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"
)

func TestPolicies(t *testing.T) {
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
		cluster := GetTestCluster(t, tt.clusterSize)
		if cluster == nil {
			t.Fatal("failed to get test cluster (can it connect to storage?)")
		}
		cluster.Start()
		defer cluster.Cleanup()

		t.Run(tt.name, func(t *testing.T) {

			var policyTemplate = `
	path "secret/%s" {
		capabilities = ["read"]
	}
	`

			// Create policies
			for i := 0; i < tt.iterations; i++ {
				client := cluster.Cores[rand.Intn(len(cluster.Cores))].Client
				policyName := fmt.Sprintf("policy-%d", i)
				err := client.Sys().PutPolicy(policyName, fmt.Sprintf(policyTemplate, "first"))
				if err != nil {
					t.Fatal(err)
				}
			}

			// Read policies
			for i := 0; i < tt.iterations; i++ {
				client := cluster.Cores[rand.Intn(len(cluster.Cores))].Client
				policyName := fmt.Sprintf("policy-%d", i)
				policy, err := client.Sys().GetPolicy(policyName)
				if err != nil {
					t.Fatal(err)
				}
				got := strings.TrimSpace(policy)
				expected := strings.TrimSpace(fmt.Sprintf(policyTemplate, "first"))
				if got != expected {
					t.Fatalf("got unexpected first policy: \n%s\n\nexpected:\n\n%s\n", got, expected)
				}
			}

			// Update policies
			for i := 0; i < tt.iterations; i++ {
				client := cluster.Cores[rand.Intn(len(cluster.Cores))].Client
				policyName := fmt.Sprintf("policy-%d", i)
				err := client.Sys().PutPolicy(policyName, fmt.Sprintf(policyTemplate, "second"))
				if err != nil {
					t.Fatal(err)
				}
			}

			// Wait for cache TTL
			time.Sleep(time.Second)

			// Read policies
			for i := 0; i < tt.iterations; i++ {
				client := cluster.Cores[rand.Intn(len(cluster.Cores))].Client
				policyName := fmt.Sprintf("policy-%d", i)
				policy, err := client.Sys().GetPolicy(policyName)
				if err != nil {
					t.Fatal(err)
				}
				got := strings.TrimSpace(policy)
				expected := strings.TrimSpace(fmt.Sprintf(policyTemplate, "second"))
				if got != expected {
					t.Fatalf("got unexpected first policy: \n%s\n\nexpected:\n\n%s\n", got, expected)
				}
			}

			// Delete policies
			for i := 0; i < tt.iterations; i++ {
				client := cluster.Cores[rand.Intn(len(cluster.Cores))].Client
				policyName := fmt.Sprintf("policy-%d", i)
				err := client.Sys().DeletePolicy(policyName)
				if err != nil {
					t.Fatal(err)
				}
			}

			// Wait for cache TTL
			time.Sleep(time.Second)

			// Read policies
			for i := 0; i < tt.iterations; i++ {
				client := cluster.Cores[rand.Intn(len(cluster.Cores))].Client
				policyName := fmt.Sprintf("policy-%d", i)
				policy, err := client.Sys().GetPolicy(policyName)
				if err != nil {
					t.Fatal(err)
				}
				if len(policy) > 0 {
					t.Fatal("expected empty policy")
				}
			}
		})
	}
}
