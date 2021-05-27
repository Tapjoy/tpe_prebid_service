package openrtb_ext

type ExtImpDV360 struct {
	Region         string `json:"region"`
	Reward         int    `json:"reward"`
	SKADNSupported bool   `json:"skadn_supported"`
	MRAIDSupported bool   `json:"mraid_supported"`
}