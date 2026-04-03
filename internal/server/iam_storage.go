package server

import (
	"sync"

	"cloud.google.com/go/iam/apiv1/iampb"
)

// IAMStorage stores IAM policies in memory, keyed by resource name.
type IAMStorage struct {
	mu       sync.RWMutex
	policies map[string]*iampb.Policy
}

// NewIAMStorage creates an empty IAM policy store.
func NewIAMStorage() *IAMStorage {
	return &IAMStorage{policies: make(map[string]*iampb.Policy)}
}

// GetPolicy returns the stored policy for the given resource.
// Returns an empty default policy if none has been set.
func (s *IAMStorage) GetPolicy(resource string) *iampb.Policy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if p, ok := s.policies[resource]; ok {
		return p
	}
	return &iampb.Policy{}
}

// SetPolicy stores the IAM policy for the given resource and returns it.
func (s *IAMStorage) SetPolicy(resource string, policy *iampb.Policy) *iampb.Policy {
	s.mu.Lock()
	defer s.mu.Unlock()
	if policy == nil {
		policy = &iampb.Policy{}
	}
	s.policies[resource] = policy
	return policy
}
