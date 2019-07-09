package es

import "time"

// TokenStoreOption is the configuration options type for token store
type TokenStoreOption func(s *TokenStore)

// WithTokenStoreTableName returns option that sets token store table name
func WithTokenStoreIndex(index string) TokenStoreOption {
	return func(s *TokenStore) {
		s.index = index
	}
}

// WithTokenStoreGCInterval returns option that sets token store garbage collection interval
func WithTokenStoreGCInterval(gcInterval time.Duration) TokenStoreOption {
	return func(s *TokenStore) {
		s.gcInterval = gcInterval
	}
}

// WithTokenStoreGCDisabled returns option that disables token store garbage collection
func WithTokenStoreGCDisabled() TokenStoreOption {
	return func(s *TokenStore) {
		s.gcDisabled = true
	}
}
