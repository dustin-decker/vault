package active

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/vault/api"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/option"
	"google.golang.org/api/storage/v1"
)

var (
	GCPProject = GetEnvStr("GCP_PROJECT", "")

	privateKeyTypeJSON = "TYPE_GOOGLE_CREDENTIALS_FILE"
	keyAlgorithmRSA2k  = "KEY_ALG_RSA_2048"
)

func TestGCPSecrets(t *testing.T) {
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
			iterations:  5,
		},
		{
			name:        "active-active HA cluster",
			clusterSize: 3,
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
				t.Fatal(err)
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
				"secret_type": "service_account_key",
				"project":     GCPProject,
				"bindings":    binding,
			})
			if err != nil {
				t.Fatal(err)
			}

			var secrets = []*api.Secret{}
			for i := 0; i < tt.iterations; i++ {
				client = cluster.Cores[rand.Intn(len(cluster.Cores))].Client

				// Obtain credential from roleset
				s, err := client.Logical().Read("gcp/key/test-ttl")
				if err != nil {
					t.Fatal(err)
				}

				// Attempt to use cred
				creds := getGoogleCredentials(t, s.Data)
				gcpClient := oauth2.NewClient(context.Background(), creds.TokenSource)
				storageAdmin, err := storage.NewService(context.Background(), option.WithHTTPClient(gcpClient))
				if err != nil {
					t.Fatalf("could not construct service client from given token: %v", err)
				}

				// Should pass: List buckets
				_, err = storageAdmin.Buckets.List(GCPProject).Do()
				if err != nil {
					t.Fatalf("expected call using authorized secret to succeed, instead got error: %v", err)
				}
				secrets = append(secrets, s)
			}

			// Wait for expiration
			time.Sleep(time.Second * 15)

			for _, s := range secrets {
				// Should fail: List buckets
				creds := getGoogleCredentials(t, s.Data)
				gcpClient := oauth2.NewClient(context.Background(), creds.TokenSource)
				storageAdmin, err := storage.NewService(context.Background(), option.WithHTTPClient(gcpClient))
				_, err = storageAdmin.Buckets.List(GCPProject).Do()
				if err == nil {
					t.Fatal("expected error from expired cred")
				}
			}
		})
	}
}

// From vault-plugin-secrets-gcp test
func getGoogleCredentials(t *testing.T, d map[string]interface{}) *google.Credentials {
	kAlg, ok := d["key_algorithm"]
	if !ok {
		t.Fatalf("expected 'key_algorithm' field to be returned")
	}
	if kAlg.(string) != keyAlgorithmRSA2k {
		t.Fatalf("expected 'key_algorithm' %s, got %v", keyAlgorithmRSA2k, kAlg)
	}

	kType, ok := d["key_type"]
	if !ok {
		t.Fatalf("expected 'key_type' field to be returned")
	}
	if kType.(string) != privateKeyTypeJSON {
		t.Fatalf("expected 'key_type' %s, got %v", privateKeyTypeJSON, kType)
	}

	keyDataRaw, ok := d["private_key_data"]
	if !ok {
		t.Fatalf("expected 'private_key_data' field to be returned")
	}
	keyJSON, err := base64.StdEncoding.DecodeString(keyDataRaw.(string))
	if err != nil {
		t.Fatalf("could not b64 decode 'private_key_data' field: %v", err)
	}

	creds, err := google.CredentialsFromJSON(context.Background(), []byte(keyJSON), iam.CloudPlatformScope)
	if err != nil {
		t.Fatalf("could not get JWT config from given 'private_key_data': %v", err)
	}
	return creds
}
