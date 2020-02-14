package active

import (
	"github.com/hashicorp/vault/command/server"
	vaulthttp "github.com/hashicorp/vault/http"
	"github.com/hashicorp/vault/vault"
	testing "github.com/mitchellh/go-testing-interface"

	physMySQL "github.com/hashicorp/vault/physical/mysql"
)

func getTestCluster(t testing.T, numServers int) *vault.TestCluster {

	storageConf := map[string]string{
		"address":  "127.0.0.1:3306",
		"username": "root",
		"password": "root",
		"database": "vault",
	}
	phys, err := physMySQL.NewMySQLBackend(storageConf, nil)
	if err != nil {
		return nil
	}
	coreConfig := &vault.CoreConfig{
		RawConfig: &server.Config{
			DisableMlock:      true,
			DisableClustering: true,
			Storage: &server.Storage{
				Type:              "mysql",
				DisableClustering: true,
				Config:            storageConf,
			},
		},
		Physical: phys,
	}
	return vault.NewTestCluster(t, coreConfig, &vault.TestClusterOptions{
		NumCores:    3,
		HandlerFunc: vaulthttp.Handler,
	})
}
