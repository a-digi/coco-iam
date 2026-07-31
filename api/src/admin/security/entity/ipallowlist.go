package entity

import (
	cocosecentity "github.com/a-digi/coco-sec/ipguard/entity"
)

// IPAllowlistRequest is the body for adding an IP to the allowlist.
type IPAllowlistRequest struct {
	IP   string `json:"ip" example:"203.0.113.7"`
	Note string `json:"note,omitempty" example:"office egress"`
}

// Swag-friendly success envelopes.

type IPAllowlistListSuccess struct {
	Success bool                             `json:"success" example:"true"`
	Message []cocosecentity.IPAllowlistEntry `json:"message"`
}

type IPAllowlistEntrySuccess struct {
	Success bool                           `json:"success" example:"true"`
	Message cocosecentity.IPAllowlistEntry `json:"message"`
}
