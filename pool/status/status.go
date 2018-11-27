package status

import (
	"context"
	"sync"
	"time"

	"github.com/vipnode/vipnode/pool/store"
)

// TODO: Support event sub?

// Host is a public view of a hosting node.
type Host struct {
	ShortID  string    `json:"short_id"`
	LastSeen time.Time `json:"last_seen"`
	Kind     string    `json:"kind"`

	// TODO: Add latest block
	// TODO: Add peers
}

func nodeHost(n store.Node) Host {
	shortID := string(n.ID)
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}
	return Host{
		ShortID:  shortID,
		LastSeen: n.LastSeen,
		Kind:     n.Kind,
	}
}

// StatusRequest is the response type for Status RPC calls.
type StatusResponse struct {
	// TimeUpdated is the time when the response was generated. Because the
	// response is cached, it can be sometime in the past.
	TimeUpdated time.Time `json:"time_updated"`

	// Version of the pool that is currently running.
	TimeStarted time.Time `json:"time_started"`

	// Version of the pool that is currently running.
	Version string `json:"version"`

	// ActiveHosts is a list of participating hosts who have been seen recently.
	ActiveHosts []Host `json:"active_hosts"`

	// Error is set if the last cache update attempt failed and the
	// timestamp was extended.
	Error error `json:"error,omitempty"`
}

// PoolStatus is a service for providing data to a pool status dashboard over
// RPC. Because status calls are unathenticated, the service only provides
// cached public consumable data.
type PoolStatus struct {
	Store store.Store

	// TimeStarted is the time when the server was started.
	TimeStarted time.Time

	// Version of the pool to report.
	Version string

	// CacheDuration is the time for responses to be cached.
	CacheDuration time.Duration

	mu         sync.RWMutex
	cachedResp *StatusResponse
}

// getStatus is an uncached version of Status
func (s *PoolStatus) getStatus() (*StatusResponse, error) {
	r := &StatusResponse{
		TimeUpdated: time.Now(),
		TimeStarted: s.TimeStarted,
		Version:     s.Version,
	}

	nodes, err := s.Store.ActiveHosts("", 0)
	if err != nil {
		r.Error = err
		return r, err
	}

	r.ActiveHosts = make([]Host, 0, len(nodes))
	for _, n := range nodes {
		r.ActiveHosts = append(r.ActiveHosts, nodeHost(n))
	}

	return r, nil
}

// Status returns the status of the pool.
func (s *PoolStatus) Status(ctx context.Context) (*StatusResponse, error) {
	s.mu.RLock()
	cachedResp := s.cachedResp
	s.mu.RUnlock()

	if cachedResp != nil && cachedResp.TimeUpdated.Add(s.CacheDuration).After(time.Now()) {
		// Cache is valid
		return cachedResp, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Did another request beat us to it?
	if s.cachedResp != cachedResp {
		return s.cachedResp, nil
	}

	// We save the status even if there is an error (to avoid an error-based DoS)
	r, err := s.getStatus()
	s.cachedResp = r
	return r, err
}