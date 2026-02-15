package market

// OIAggregator is a placeholder that would aggregate open interest rankings from venues.
type OIAggregator struct{}

// NewOIAggregator constructs a stub aggregator.
func NewOIAggregator() *OIAggregator {
	return &OIAggregator{}
}

// GetOIRanking returns an empty ranking for now.
func (a *OIAggregator) GetOIRanking(symbols []string) ([]map[string]interface{}, error) {
	return []map[string]interface{}{}, nil
}
