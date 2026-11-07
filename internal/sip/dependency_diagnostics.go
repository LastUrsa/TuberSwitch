package sip

import "context"

func (s *Service) diagnosticDependencies(ctx context.Context) any {
	provider, ok := s.controller.(StatusDetailsProvider)
	if !ok {
		return map[string]any{"obs": map[string]any{"state": "unknown"}}
	}
	details, err := provider.SIPStatusDetails(ctx)
	if err != nil {
		return map[string]any{"obs": map[string]any{"state": "unavailable", "errorCode": "UNAVAILABLE"}}
	}
	return map[string]any{"obs": map[string]any{"connected": details.OBSConnected, "state": details.OBSConnectionState, "observedAt": details.OBSObservedAt, "lastConnectedAt": details.OBSLastConnectedAt, "lastDisconnectedAt": details.OBSLastDisconnectedAt, "lastDisconnectCode": details.OBSLastDisconnectCode, "nextReconnectAt": details.OBSNextReconnectAt}}
}
