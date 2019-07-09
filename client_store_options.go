package es

// TokenStoreOption is the configuration options type for token store
type ClientStoreOption func(s *ClientStore)

// WithTokenStoreTableName returns option that sets token store table name
func WithClientStoreIndex(index string) ClientStoreOption {
	return func(s *ClientStore) {
		s.index = index
	}
}