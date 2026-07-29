package entity

// FetchGeoIPResponse is POST .../attacks/{id}/geoip's response.
// Matched is false (a normal outcome, not an error) when the episode's
// IP is loopback/private or GeoIP simply has no current coverage for
// it — nothing is persisted in that case, so the caller can retry
// later. GeoIPInfo is only ever populated when Matched is true.
type FetchGeoIPResponse struct {
	Matched   bool   `json:"matched" example:"true"`
	GeoIPInfo string `json:"geoip_info,omitempty" example:"{\"country_code\":\"DE\",\"country\":\"Germany\",\"asn\":3320,\"as_org\":\"Deutsche Telekom AG\"}"`
}

// Swag-friendly success envelope.
type FetchGeoIPSuccess struct {
	Success bool               `json:"success" example:"true"`
	Message FetchGeoIPResponse `json:"message"`
}
