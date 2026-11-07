package sip

import (
	"TuberSwitch/internal/appdetect"
	"errors"
)

const ProtocolVersion = 1

const (
	ProfilesCapability = "profiles"
	RedeemsCapability  = "redeems"
)

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
	ErrRedeemNotFound  = errors.New("RedeemNotFound")
	ErrForbidden       = errors.New("Forbidden")
)

type AppInfo struct {
	AppID    string
	Name     string
	Version  string
	Mode     string
	Protocol int
}

type Profile struct {
	ID   string
	Name string
	Mode string
}

type AppResponse struct {
	AppID           string   `json:"appId"`
	AppName         string   `json:"appName"`
	Name            string   `json:"name"`
	Version         string   `json:"version"`
	Mode            string   `json:"mode"`
	ProtocolVersion int      `json:"protocolVersion"`
	Capabilities    []string `json:"capabilities"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type CapabilitiesResponse struct {
	ProtocolVersion         int      `json:"protocolVersion"`
	Capabilities            []string `json:"capabilities"`
	SupportsProfiles        bool     `json:"supportsProfiles"`
	SupportsStatusReporting bool     `json:"supportsStatusReporting"`
	SupportsRedeems         bool     `json:"supportsRedeems"`
}

type StatusResponse struct {
	AppDetection            appdetect.Diagnostics `json:"appDetection"`
	OBSObservedAt           string                `json:"obsObservedAt,omitempty"`
	OBSLastConnectedAt      string                `json:"obsLastConnectedAt,omitempty"`
	OBSLastDisconnectedAt   string                `json:"obsLastDisconnectedAt,omitempty"`
	OBSLastDisconnectCode   string                `json:"obsLastDisconnectCode,omitempty"`
	OBSNextReconnectAt      string                `json:"obsNextReconnectAt,omitempty"`
	State                   string                `json:"state"`
	Message                 string                `json:"message"`
	Healthy                 bool                  `json:"healthy"`
	ActiveProfile           string                `json:"activeProfile,omitempty"`
	ActiveProfileID         string                `json:"activeProfileId,omitempty"`
	ActiveProfileName       string                `json:"activeProfileName,omitempty"`
	Mode                    string                `json:"mode,omitempty"`
	ActiveMode              string                `json:"activeMode,omitempty"`
	OBSSummary              string                `json:"obsSummary,omitempty"`
	OBSConnected            bool                  `json:"obsConnected"`
	OBSConnectionState      string                `json:"obsConnectionState,omitempty"`
	ActiveScene             string                `json:"activeScene,omitempty"`
	ActiveSource            string                `json:"activeSource,omitempty"`
	RedeemsEnabled          bool                  `json:"redeemsEnabled"`
	RedeemCount             int                   `json:"redeemCount"`
	ManageableRedeemCount   int                   `json:"manageableRedeemCount"`
	UnmanageableRedeemCount int                   `json:"unmanageableRedeemCount"`
	AppDetectionStatus      string                `json:"appDetectionStatus,omitempty"`
	AppDetectionEnabled     bool                  `json:"appDetectionEnabled"`
	CurrentModeLabel        string                `json:"currentModeLabel,omitempty"`
	ActiveProfileLastUsed   string                `json:"activeProfileLastUsed,omitempty"`
}

type StatusDetails struct {
	AppDetection                                                                                        appdetect.Diagnostics
	OBSObservedAt, OBSLastConnectedAt, OBSLastDisconnectedAt, OBSLastDisconnectCode, OBSNextReconnectAt string
	OBSConnected                                                                                        bool
	OBSConnectionState                                                                                  string
	OBSSummary                                                                                          string
	ActiveScene                                                                                         string
	ActiveSource                                                                                        string
	RedeemsEnabled                                                                                      bool
	RedeemCount                                                                                         int
	ManageableRedeemCount                                                                               int
	UnmanageableRedeemCount                                                                             int
	AppDetectionEnabled                                                                                 bool
	AppDetectionStatus                                                                                  string
	CurrentModeLabel                                                                                    string
	ActiveProfileLastUsed                                                                               string
}

type ProfilesResponse struct {
	Profiles []string `json:"profiles"`
}

type Redeem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Enabled   bool   `json:"enabled"`
}

type RedeemsResponse struct {
	Redeems []Redeem `json:"redeems"`
}

type UpdateRedeemRequest struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}

type UpdateRedeemsRequest struct {
	Redeems []UpdateRedeemRequest `json:"redeems"`
}

type SuccessResponse struct {
	Success bool `json:"success"`
}

type ActivateProfileRequest struct {
	Profile string `json:"profile"`
}

type ProfileActivationResponse struct {
	Error              string             `json:"error,omitempty"`
	ErrorCode          string             `json:"errorCode,omitempty"`
	OperationOutcome   string             `json:"operationOutcome,omitempty"`
	RequestedProfileID string             `json:"requestedProfileId,omitempty"`
	ActiveProfileID    string             `json:"activeProfileId,omitempty"`
	Results            []DiagnosticResult `json:"results,omitempty"`

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
