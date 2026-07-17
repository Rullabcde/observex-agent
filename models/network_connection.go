package models

type NetworkConnectionInfo struct {
	PID        int     `json:"pid"`
	Name       string  `json:"name"`
	RemoteIP   string  `json:"remoteIp"`
	RemotePort int     `json:"remotePort"`
	Protocol   string  `json:"protocol"`
	RxRate     float64 `json:"rxRate"`
	TxRate     float64 `json:"txRate"`
}
