package data

import "strings"

// supportedVietnamMapNames contains every map identifier currently returned
// by known HLL: Vietnam RCON builds for the six calibrated maps.
var supportedVietnamMapNames = []string{
	"wdeva_warfare_day",
	"wdeva",
	"Vạn Tường",
	"wdevb_warfare_day",
	"wdevb",
	"Quảng Ngãi",
	"Quang Ngai",
	"wdevc_warfare_day",
	"wdevc",
	"Huế Outskirts",
	"Hue Outskirts",
	"wdevd_warfare_day",
	"wdevd",
	"Đăk Tô Airfield",
	"Dak To Airfield",
	"wdeve_warfare_day",
	"wdeve",
	"Cam Ranh Port",
	"wdevf_warfare_day",
	"wdevf",
	"Thanh Hòa Bridge",
	"Thanh Hoa Bridge",
}

// IsSupportedVietnamMapName reports whether value is one of the calibrated
// HLL: Vietnam map aliases. Matching is case-insensitive and ignores outer
// whitespace to accommodate differences between RCON builds.
func IsSupportedVietnamMapName(value string) bool {
	value = strings.TrimSpace(value)
	for _, supported := range supportedVietnamMapNames {
		if strings.EqualFold(value, supported) {
			return true
		}
	}
	return false
}
