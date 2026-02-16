package models

type ServiceInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Status      string `json:"status"`    // running, stopped, exited, dead, unknown
	StartType   string `json:"startType"` // auto, manual, disabled, unknown
}
