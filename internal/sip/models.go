package sip

import "errors"

const ProtocolVersion = "1.1"

const (
	StandaloneMode = "standalone"
	ServiceMode    = "service"
)

const (
	HealthReady = "ready"
	HealthError = "error"
)

const (
	StatusReady = "ready"
	StatusError = "error"
)

var (
	ErrInvalidRequest  = errors.New("InvalidRequest")
	ErrProfileNotFound = errors.New("ProfileNotFound")
	ErrForbidden       = errors.New("Forbidden")
)

type AppInfo struct {
	AppID    string
	Name     string
	Version  string
	Mode     string
	Protocol string
}

type Profile struct {
	ID   string
	Name string
	Mode string
}

type AppResponse struct {
	AppID           string `json:"appId"`
	Name            string `json:"name"`
	Version         string `json:"version"`
	Mode            string `json:"mode"`
	ProtocolVersion string `json:"protocolVersion"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type CapabilitiesResponse struct {
	SupportsProfiles        bool `json:"supportsProfiles"`
	SupportsStatusReporting bool `json:"supportsStatusReporting"`
}

type StatusResponse struct {
	State           string `json:"state"`
	Message         string `json:"message"`
	Healthy         bool   `json:"healthy"`
	ActiveProfile   string `json:"activeProfile,omitempty"`
	ActiveProfileID string `json:"activeProfileId,omitempty"`
	ActiveMode      string `json:"activeMode,omitempty"`
}

type ProfilesResponse struct {
	Profiles []string `json:"profiles"`
}

type ActivateProfileRequest struct {
	Profile string `json:"profile"`
}

type ProfileActivationResponse struct {
	Success   bool   `json:"success"`
	Profile   string `json:"profile,omitempty"`
	ProfileID string `json:"profileId,omitempty"`
}

type CurrentProfileResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}
