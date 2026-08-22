package maprules_test

import (
	"testing"

	"github.com/floriansw/go-hll-rcon/rconv2"
	"github.com/kilan558/hllv-geofences/data"
	"github.com/kilan558/hllv-geofences/maprules"
)

func TestAllVietnamMapsUseTheConfiguredProfileThresholds(t *testing.T) {
	tests := []struct {
		name    string
		apply   func(*data.Config, maprules.Variant) error
		session func(int) *rconv2.GetSessionInfoResponse
	}{
		{name: "Vạn Tường", apply: maprules.ApplyVanTuong, session: vanTuongSession},
		{name: "Quảng Ngãi", apply: maprules.ApplyQuangNgai, session: quangNgaiSession},
		{name: "Huế Outskirts", apply: maprules.ApplyHueOutskirts, session: hueOutskirtsSession},
		{name: "Đăk Tô Airfield", apply: maprules.ApplyDakToAirfield, session: dakToAirfieldSession},
		{name: "Cam Ranh Port", apply: maprules.ApplyCamRanhPort, session: camRanhPortSession},
		{name: "Thanh Hòa Bridge", apply: maprules.ApplyThanhHoa, session: thanhHoaSession},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, threshold := range []struct {
				variant       maprules.Variant
				lastProtected int
				firstOpen     int
			}{
				{variant: maprules.VariantMidcap, lastProtected: 60, firstOpen: 61},
				{variant: maprules.VariantLastcap, lastProtected: 90, firstOpen: 91},
			} {
				config := data.Config{Servers: []data.Server{{
					Host: "192.0.2.1", Port: 7779, Password: "secret",
				}}}
				if err := tc.apply(&config, threshold.variant); err != nil {
					t.Fatalf("apply %s: %v", threshold.variant, err)
				}
				fences := append(config.Servers[0].AxisFence, config.Servers[0].AlliesFence...)
				if got := matchingCount(fences, tc.session(threshold.lastProtected)); got == 0 {
					t.Fatalf("%s has no active fences at %d players", threshold.variant, threshold.lastProtected)
				}
				if got := matchingCount(fences, tc.session(threshold.firstOpen)); got != 0 {
					t.Fatalf("%s has %d active fences at %d players, want 0", threshold.variant, got, threshold.firstOpen)
				}
			}
		})
	}
}
