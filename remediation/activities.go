package remediation

import (
	"context"
	"fmt"
	"sync"
)

// Remediator performs the real infrastructure change. Implementations MUST be idempotent
// per key: Temporal may run an activity more than once (retries, worker crash after
// the effect but before the result was recorded).
type Remediator interface {
	Restart(service, key string) error
	Rollback(service, key string) error
	Healthy(service string) bool
}

type Activities struct{ R Remediator }

func (a *Activities) Diagnose(_ context.Context, in Incident) (string, error) {
	return fmt.Sprintf("service %s flagged unhealthy; disruptive=%v", in.Service, in.Disruptive), nil
}

func (a *Activities) Restart(_ context.Context, service, key string) error {
	return a.R.Restart(service, key)
}
func (a *Activities) Rollback(_ context.Context, service, key string) error {
	return a.R.Rollback(service, key)
}

func (a *Activities) Verify(_ context.Context, service string) error {
	if !a.R.Healthy(service) {
		return fmt.Errorf("service %s still unhealthy after remediation", service)
	}
	return nil
}

// MemRemediator is an in-memory demo implementation that shows idempotency.
type MemRemediator struct {
	mu      sync.Mutex
	Applied map[string]int // key -> times the real effect ran
}

func NewMemRemediator() *MemRemediator { return &MemRemediator{Applied: map[string]int{}} }

func (m *MemRemediator) Restart(service, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, done := m.Applied[key]; done {
		return nil // already applied: repeat is a no-op
	}
	m.Applied[key] = 1
	return nil
}
func (m *MemRemediator) Rollback(service, key string) error { return nil }
func (m *MemRemediator) Healthy(string) bool                { return true }
