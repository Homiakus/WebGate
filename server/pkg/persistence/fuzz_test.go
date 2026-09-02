package persistence

import (
	"encoding/json"
	"testing"

	"github.com/Homiakus/WebGate/server/pkg/config"
	"github.com/Homiakus/WebGate/server/pkg/domain"
)

func FuzzDurableServerConfigUnmarshal(f *testing.F) {
	seed := []byte(`{"server":{"data_port":43110,"admin_port":43119,"host":"127.0.0.1"}}`)
	f.Add(seed)
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"server":null}`))
	f.Add([]byte(`{"relay_nodes":[{"name":"r1","address":"127.0.0.1","port":4444}]}`))
	f.Add([]byte(`corrupted payload`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var cfg config.DurableServerConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			return
		}
		// Invariant: checksumFor must not panic on any byte slice
		_ = checksumFor("control_config", "singleton", data)
	})
}

func FuzzAuditEventUnmarshal(f *testing.F) {
	seed := []byte(`{"id":"evt_1","action":"CREATE_SERVICE","actor":"admin","timestamp":"2026-09-01T00:00:00Z"}`)
	f.Add(seed)
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"id":""}`))
	f.Add([]byte(`{"id":"123","action":"","timestamp":"invalid-time"}`))
	f.Add([]byte(`invalid bytes`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var event domain.AuditEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return
		}
		_ = checksumFor("audit", event.ID, data)
	})
}
