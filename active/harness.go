package active

import (
	"os"
	"strconv"
	"time"

	"github.com/hashicorp/vault/builtin/logical/pki"
	"github.com/hashicorp/vault/command/server"
	vaulthttp "github.com/hashicorp/vault/http"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/hashicorp/vault/vault"
	testing "github.com/mitchellh/go-testing-interface"

	physMySQL "github.com/hashicorp/vault/physical/mysql"
)

var (
	StorageConf = map[string]string{
		"address":  GetEnvStr("DATABASE_ADDR", "127.0.0.1:3306"),
		"username": GetEnvStr("DATABASE_USERNAME", "root"),
		"password": GetEnvStr("DATABASE_PASSWORD", "root"),
	}

	ClusterSize = GetEnvInt("CLUSTER_SIZE", 3)
)

func GetEnvStr(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		value = fallback
	}
	return value
}

func GetEnvInt(key string, fallback int) int {
	value, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}
	intVal, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return intVal
}

func GetTestCluster(t testing.T, cluserSize int) *vault.TestCluster {
	StorageConf["database"] = time.Now().String()
	phys, err := physMySQL.NewMySQLBackend(StorageConf, nil)
	if err != nil {
		return nil
	}
	coreConfig := &vault.CoreConfig{
		RawConfig: &server.Config{
			DisableMlock:      true,
			DisableClustering: true,
			CacheTTL:          time.Microsecond,
			Storage: &server.Storage{
				Type:              "mysql",
				DisableClustering: true,
				Config:            StorageConf,
			},
		},
		Physical: phys,
		LogicalBackends: map[string]logical.Factory{
			"pki": pki.Factory,
		},
	}
	return vault.NewTestCluster(t, coreConfig, &vault.TestClusterOptions{
		NumCores:    cluserSize,
		HandlerFunc: vaulthttp.Handler,
	})
}
