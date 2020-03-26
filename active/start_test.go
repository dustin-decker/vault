package active

import (
	"testing"
)

func TestStart(t *testing.T) {
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
			// Add your tests below here //

		})
	}
}
