package sip

import (
	"context"
	"strings"
)

type ProfileController interface {
	SIPProfiles(context.Context) ([]Profile, error)
	SIPCurrentProfile(context.Context) (Profile, error)
	SIPActivateProfile(context.Context, string) (Profile, error)
}

type RedeemController interface {
	SIPRedeems(context.Context) ([]Redeem, error)
	SIPSetRedeems(context.Context, []UpdateRedeemRequest) error
}

type StatusDetailsProvider interface {
	SIPStatusDetails(context.Context) (StatusDetails, error)
}

type Service struct {
	info       AppInfo
	controller ProfileController
}

func NewService(info AppInfo, controller ProfileController) *Service {
	if info.Protocol == 0 {
		info.Protocol = ProtocolVersion
	}
	if info.Mode == "" {
		info.Mode = StandaloneMode
	}
	return &Service{info: info, controller: controller}
}

func (s *Service) App(context.Context) (AppResponse, error) {
	if s == nil {
		return AppResponse{}, nil
	}
	return AppResponse{
		AppID:           s.info.AppID,
		AppName:         s.info.Name,
		Name:            s.info.Name,
		Version:         s.info.Version,
		Mode:            normalizeMode(s.info.Mode),
		ProtocolVersion: s.info.Protocol,
		Capabilities:    sipCapabilities(),
	}, nil
}

func (s *Service) Health(ctx context.Context) (HealthResponse, error) {
	if s == nil || s.controller == nil {
		return HealthResponse{Status: HealthError, Message: "SIP service is unavailable."}, nil
	}
	if _, err := s.controller.SIPProfiles(ctx); err != nil {
		return HealthResponse{Status: HealthError, Message: "TuberSwitch profiles are unavailable."}, nil
	}
	return HealthResponse{Status: HealthReady, Message: "TuberSwitch operational"}, nil
}

func (s *Service) Capabilities(context.Context) (CapabilitiesResponse, error) {
	return CapabilitiesResponse{
		ProtocolVersion:         ProtocolVersion,
		Capabilities:            sipCapabilities(),
		SupportsProfiles:        true,
		SupportsStatusReporting: true,
		SupportsRedeems:         true,
	}, nil
}

func (s *Service) Status(ctx context.Context) (StatusResponse, error) {
	health, _ := s.Health(ctx)
	response := StatusResponse{
		State:   StatusReady,
		Message: "Profile active",
		Healthy: health.Status == HealthReady,
	}
	if health.Status != HealthReady {
		response.State = StatusError
		response.Message = health.Message
		return response, nil
	}
	if s == nil || s.controller == nil {
		return response, nil
	}
	profile, err := s.controller.SIPCurrentProfile(ctx)
	if err != nil {
		response.State = StatusError
		response.Message = "Active profile unavailable."
		response.Healthy = false
		return response, nil
	}
	response.ActiveProfile = profile.Name
	response.ActiveProfileID = profile.ID
	response.ActiveProfileName = profile.Name
	response.Mode = normalizeProfileMode(profile.Mode)
	response.ActiveMode = strings.ToLower(profile.Mode)
	if provider, ok := s.controller.(StatusDetailsProvider); ok {
		if details, err := provider.SIPStatusDetails(ctx); err == nil {
			response.OBSSummary = details.OBSSummary
			response.OBSConnected = details.OBSConnected
			response.ActiveScene = details.ActiveScene
			response.ActiveSource = details.ActiveSource
			response.RedeemsEnabled = details.RedeemsEnabled
			response.RedeemCount = details.RedeemCount
			response.ManageableRedeemCount = details.ManageableRedeemCount
			response.UnmanageableRedeemCount = details.UnmanageableRedeemCount
			response.AppDetectionEnabled = details.AppDetectionEnabled
			response.AppDetectionStatus = details.AppDetectionStatus
			response.CurrentModeLabel = details.CurrentModeLabel
			response.ActiveProfileLastUsed = details.ActiveProfileLastUsed
		}
	}
	return response, nil
}

func (s *Service) Redeems(ctx context.Context) (RedeemsResponse, error) {
	if s == nil || s.controller == nil {
		return RedeemsResponse{}, nil
	}
	controller, ok := s.controller.(RedeemController)
	if !ok {
		return RedeemsResponse{}, nil
	}
	redeems, err := controller.SIPRedeems(ctx)
	if err != nil {
		return RedeemsResponse{}, err
	}
	return RedeemsResponse{Redeems: redeems}, nil
}

func (s *Service) SetRedeems(ctx context.Context, updates []UpdateRedeemRequest) (SuccessResponse, error) {
	if len(updates) == 0 {
		return SuccessResponse{}, ErrInvalidRequest
	}
	if s == nil || s.controller == nil {
		return SuccessResponse{}, ErrInvalidRequest
	}
	controller, ok := s.controller.(RedeemController)
	if !ok {
		return SuccessResponse{}, ErrInvalidRequest
	}
	for _, update := range updates {
		if strings.TrimSpace(update.ID) == "" {
			return SuccessResponse{}, ErrInvalidRequest
		}
	}
	if err := controller.SIPSetRedeems(ctx, updates); err != nil {
		return SuccessResponse{}, err
	}
	return SuccessResponse{Success: true}, nil
}

func (s *Service) Profiles(ctx context.Context) (ProfilesResponse, error) {
	if s == nil || s.controller == nil {
		return ProfilesResponse{}, nil
	}
	profiles, err := s.controller.SIPProfiles(ctx)
	if err != nil {
		return ProfilesResponse{}, err
	}
	names := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		if strings.TrimSpace(profile.Name) != "" {
			names = append(names, profile.Name)
		}
	}
	return ProfilesResponse{Profiles: names}, nil
}

func (s *Service) CurrentProfile(ctx context.Context) (CurrentProfileResponse, error) {
	if s == nil || s.controller == nil {
		return CurrentProfileResponse{}, nil
	}
	profile, err := s.controller.SIPCurrentProfile(ctx)
	if err != nil {
		return CurrentProfileResponse{}, err
	}
	return CurrentProfileResponse{ID: profile.ID, Name: profile.Name}, nil
}

func (s *Service) ActivateProfile(ctx context.Context, name string) (ProfileActivationResponse, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ProfileActivationResponse{}, ErrInvalidRequest
	}
	if s == nil || s.controller == nil {
		return ProfileActivationResponse{}, ErrProfileNotFound
	}
	profile, err := s.controller.SIPActivateProfile(ctx, name)
	if err != nil {
		return ProfileActivationResponse{}, err
	}
	return ProfileActivationResponse{
		Success:   true,
		Profile:   profile.Name,
		ProfileID: profile.ID,
	}, nil
}

func normalizeMode(mode string) string {
	if strings.EqualFold(mode, ServiceMode) {
		return ServiceMode
	}
	return StandaloneMode
}

func normalizeProfileMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "3d":
		return "3d"
	case "png":
		return "png"
	default:
		return "unknown"
	}
}

func sipCapabilities() []string {
	return []string{ProfilesCapability, RedeemsCapability}
}
