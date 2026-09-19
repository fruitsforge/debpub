package lock

import (
	"encoding/json"
	"time"
)

// LockInfo represents the distributed lock metadata stored in the repository.
type LockInfo struct {
	// Holder identifies the entity holding the lock (e.g. runner hostname or user).
	Holder string `json:"holder"`

	// MachineID represents the host or container hostname running the job.
	MachineID string `json:"machine_id,omitempty"`

	// PID of the running process if available.
	PID int `json:"pid,omitempty"`

	// CreatedAt is the UTC timestamp when the lock was acquired.
	CreatedAt time.Time `json:"created_at"`

	// ExpiresAt is the UTC timestamp after which the lock is considered stale and breakable.
	ExpiresAt time.Time `json:"expires_at"`

	// Reason explains the purpose of the lock (e.g., package being published).
	Reason string `json:"reason,omitempty"`

	// Metadata holds arbitrary key-value pairs for CI/CD tracking (e.g. pipeline_id, git_commit).
	Metadata map[string]string `json:"metadata,omitempty"`
}

// UnmarshalJSON implements custom alias resolution for backwards compatibility with deb-s3 and other tools.
func (l *LockInfo) UnmarshalJSON(data []byte) error {
	type Alias LockInfo
	aux := &struct {
		LockedBy   string     `json:"locked_by"`
		Timestamp  *time.Time `json:"timestamp"`
		Expiration *time.Time `json:"expiration"`
		*Alias
	}{
		Alias: (*Alias)(l),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	if l.Holder == "" && aux.LockedBy != "" {
		l.Holder = aux.LockedBy
	}
	if l.CreatedAt.IsZero() && aux.Timestamp != nil {
		l.CreatedAt = *aux.Timestamp
	}
	if l.ExpiresAt.IsZero() && aux.Expiration != nil {
		l.ExpiresAt = *aux.Expiration
	}
	return nil
}
