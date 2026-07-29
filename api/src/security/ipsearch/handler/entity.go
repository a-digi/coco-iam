package handler

// IPSearchResult is one IP's geoip lookup outcome. Matched is false
// when geoip.db has no coverage for that address (loopback/private, or
// simply no GeoLite2 allocation data) — a normal outcome, not an error.
type IPSearchResult struct {
	IP          string `json:"ip" example:"94.154.43.188"`
	Matched     bool   `json:"matched" example:"true"`
	CountryCode string `json:"country_code,omitempty" example:"DE"`
	Country     string `json:"country,omitempty" example:"Germany"`
	Subdivision string `json:"subdivision,omitempty" example:"Berlin"`
	City        string `json:"city,omitempty" example:"Berlin"`
	PostalCode  string `json:"postal_code,omitempty" example:"10115"`
	ASN         uint   `json:"asn,omitempty" example:"3320"`
	ASOrg       string `json:"as_org,omitempty" example:"Deutsche Telekom AG"`
}

// IPSearchResponse is the GET /admin/security/geoip/search response
// shape. A complete IP query yields exactly one Results entry; a
// partial/prefix query yields up to `limit` known-IP suggestions
// (from recorded attack/scan history), each resolved through the same
// live geoip lookup as the complete-IP case.
type IPSearchResponse struct {
	Query   string           `json:"query" example:"94.154.43."`
	Results []IPSearchResult `json:"results"`
}

// Swag-friendly success envelope.
type IPSearchSuccess struct {
	Success bool             `json:"success" example:"true"`
	Message IPSearchResponse `json:"message"`
}
