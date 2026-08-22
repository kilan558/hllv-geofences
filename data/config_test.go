package data_test

import (
	"github.com/floriansw/go-hll-rcon/rconv2"
	"github.com/kilan558/hllv-geofences/data"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

var _ = Describe("Config", func() {
	Describe("Persistence", func() {
		It("loads an existing config without rewriting it", func() {
			l := slog.New(slog.NewTextHandler(os.Stdout, nil))
			f, err := os.CreateTemp(os.TempDir(), "config")
			Expect(err).ToNot(HaveOccurred())
			defer os.Remove(f.Name())
			contents := []byte("Servers: []\n")
			Expect(os.WriteFile(f.Name(), contents, 0600)).ToNot(HaveOccurred())
			c, err := data.NewConfig(f.Name(), l)
			Expect(err).ToNot(HaveOccurred())
			Expect(c.Servers).To(BeEmpty())
			after, err := os.ReadFile(f.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(after).To(Equal(contents))
		})

		It("saves credentials with owner-only permissions", func() {
			l := slog.New(slog.NewTextHandler(os.Stdout, nil))
			f, err := os.CreateTemp(os.TempDir(), "config")
			Expect(err).ToNot(HaveOccurred())
			defer os.Remove(f.Name())
			Expect(os.WriteFile(f.Name(), []byte("Servers: []\n"), 0644)).ToNot(HaveOccurred())
			c, err := data.NewConfig(f.Name(), l)
			Expect(err).ToNot(HaveOccurred())
			Expect(c.Save()).To(Succeed())
			stat, err := os.Stat(f.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0600)))
		})

		It("preserves the existing owner when root updates a runtime config", func() {
			if os.Geteuid() != 0 {
				Skip("ownership preservation for a different UID requires root")
			}

			l := slog.New(slog.NewTextHandler(os.Stdout, nil))
			f, err := os.CreateTemp(os.TempDir(), "config-owner")
			Expect(err).ToNot(HaveOccurred())
			defer os.Remove(f.Name())
			Expect(os.WriteFile(f.Name(), []byte("Servers: []\n"), 0600)).ToNot(HaveOccurred())
			Expect(os.Chown(f.Name(), 1000, 1000)).To(Succeed())

			c, err := data.NewConfig(f.Name(), l)
			Expect(err).ToNot(HaveOccurred())
			Expect(c.Save()).To(Succeed())

			stat, err := os.Stat(f.Name())
			Expect(err).ToNot(HaveOccurred())
			owner, ok := stat.Sys().(*syscall.Stat_t)
			Expect(ok).To(BeTrue())
			Expect(owner.Uid).To(Equal(uint32(1000)))
			Expect(owner.Gid).To(Equal(uint32(1000)))
		})
	})

	Describe("Fence", func() {
		Context("Includes", func() {
			It("returns false when not includes", func() {
				Expect(data.Fence{
					X:       Pointer("G"),
					Y:       Pointer(4),
					Numpads: []int{4, 5, 6},
				}.Includes(rconv2.Grid{X: "H", Y: 1, Numpad: 7})).To(BeFalse())
			})

			It("includes fence when direct match", func() {
				Expect(data.Fence{
					X:       Pointer("G"),
					Y:       Pointer(4),
					Numpads: []int{4, 5, 6},
				}.Includes(rconv2.Grid{X: "G", Y: 4, Numpad: 5})).To(BeTrue())
			})

			It("includes fence when matching whole line X-axis", func() {
				Expect(data.Fence{
					X:       Pointer("G"),
					Numpads: []int{4, 5, 6},
				}.Includes(rconv2.Grid{X: "G", Y: 8, Numpad: 5})).To(BeTrue())
			})

			It("includes fence when matching whole line Y-axis", func() {
				Expect(data.Fence{
					Y:       Pointer(5),
					Numpads: []int{4, 5, 6},
				}.Includes(rconv2.Grid{X: "A", Y: 5, Numpad: 5})).To(BeTrue())
			})

			It("includes fence when no numpad specified", func() {
				Expect(data.Fence{
					X: Pointer("G"),
					Y: Pointer(5),
				}.Includes(rconv2.Grid{X: "G", Y: 5, Numpad: 7})).To(BeTrue())
			})
		})

		Context("Matches", func() {
			var si *rconv2.GetSessionInfoResponse

			BeforeEach(func() {
				si = &rconv2.GetSessionInfoResponse{
					MapName:     "CARENTAN",
					GameMode:    rconv2.GetSessionInfoGameModeWarfare,
					PlayerCount: 40,
				}
			})

			It("matches when no condition", func() {
				Expect(data.Fence{}.Matches(si)).To(BeTrue())
			})

			It("map with same name and mode", func() {
				Expect(data.Fence{Condition: &data.Condition{
					Equals: map[string][]string{
						"map_name":  {si.MapName},
						"game_mode": {si.GameMode.String()},
					},
				}}.Matches(si)).To(BeTrue())
			})

			It("matches the uppercase Vietnam display name returned by RCON", func() {
				si.MapName = "VẠN TƯỜNG"
				Expect(data.Fence{Condition: &data.Condition{
					Equals: map[string][]string{
						"map_name":  {"Vạn Tường"},
						"game_mode": {"Warfare"},
					},
				}}.Matches(si)).To(BeTrue())
			})

			It("does not match with wrong game mode", func() {
				Expect(data.Fence{Condition: &data.Condition{
					Equals: map[string][]string{
						"map_name":  {si.MapName},
						"game_mode": {"Skirmish"},
					},
				}}.Matches(si)).To(BeFalse())
			})

			It("does not match with wrong map name", func() {
				Expect(data.Fence{Condition: &data.Condition{
					Equals: map[string][]string{
						"map_name":  {"TOBRUK"},
						"game_mode": {si.GameMode.String()},
					},
				}}.Matches(si)).To(BeFalse())
			})

			DescribeTable("when less than players than", func(pc int, expected bool) {
				Expect(data.Fence{Condition: &data.Condition{
					LessThan: map[string]int{
						"player_count": pc,
					},
				}}.Matches(si)).To(Equal(expected))
			},
				Entry("more players", 20, false),
				Entry("less players", 60, true),
				Entry("equal number of players", 40, false),
			)

			DescribeTable("when greater than players than", func(pc int, expected bool) {
				Expect(data.Fence{Condition: &data.Condition{
					GreaterThan: map[string]int{
						"player_count": pc,
					},
				}}.Matches(si)).To(Equal(expected))
			},
				Entry("more players", 20, true),
				Entry("less players", 60, false),
				Entry("equal number of players", 40, false),
			)
		})
	})

	Describe("Grid geometry", func() {
		It("uses the HLL:V 200 metre default while allowing map calibration", func() {
			geometry, err := (data.GridGeometry{}).Normalized()
			Expect(err).ToNot(HaveOccurred())
			Expect(geometry.SectorSize).To(Equal(float64(20000)))

			geometry, err = (data.GridGeometry{
				SectorSize:    19840,
				CenterOffsetX: 150,
				CenterOffsetY: -110,
			}).Normalized()
			Expect(err).ToNot(HaveOccurred())
			Expect(geometry.SectorSize).To(Equal(float64(19840)))
			Expect(geometry.CenterOffsetX).To(Equal(float64(150)))
		})

		It("rejects an invalid sector size", func() {
			_, err := (data.GridGeometry{SectorSize: -1}).Normalized()
			Expect(err).To(MatchError(ContainSubstring("SectorSize")))
		})
	})

	Describe("Validation", func() {
		It("accepts a prepared server with empty fences", func() {
			config := data.Config{Servers: []data.Server{{
				Host:        "192.0.2.1",
				Port:        7779,
				Password:    "secret",
				AxisFence:   []data.Fence{},
				AlliesFence: []data.Fence{},
			}}}
			Expect(config.Validate()).To(Succeed())
		})

		DescribeTable("rejects unsafe server data", func(server data.Server, message string) {
			config := data.Config{Servers: []data.Server{server}}
			Expect(config.Validate()).To(MatchError(ContainSubstring(message)))
		},
			Entry("missing host", data.Server{Port: 7779, Password: "secret"}, "Host"),
			Entry("invalid port", data.Server{Host: "192.0.2.1", Port: 0, Password: "secret"}, "Port"),
			Entry("missing password", data.Server{Host: "192.0.2.1", Port: 7779}, "Password"),
			Entry("invalid grid", data.Server{Host: "192.0.2.1", Port: 7779, Password: "secret", Grid: data.GridGeometry{SectorSize: -1}}, "SectorSize"),
		)
	})
})

func Pointer[T any](v T) *T {
	return &v
}

func TestConfigValidationRejectsNonVietnamFence(t *testing.T) {
	tests := []struct {
		name    string
		fence   data.Fence
		wantErr string
	}{
		{
			name: "Classic HLL map",
			fence: validationFence(
				[]string{"Carentan"},
				[]string{"Warfare"},
			),
			wantErr: "unsupported Vietnam map",
		},
		{
			name: "missing map condition",
			fence: data.Fence{Condition: &data.Condition{Equals: map[string][]string{
				"game_mode": {"Warfare"},
			}}},
			wantErr: "map_name",
		},
		{
			name: "missing condition",
			fence: data.Fence{
				X: Pointer("A"),
				Y: Pointer(3),
			},
			wantErr: "Condition",
		},
		{
			name: "unsupported game mode",
			fence: validationFence(
				[]string{"wdeva_warfare_day"},
				[]string{"Offensive"},
			),
			wantErr: "Warfare",
		},
		{
			name: "missing game mode",
			fence: data.Fence{Condition: &data.Condition{Equals: map[string][]string{
				"map_name": {"wdeva_warfare_day"},
			}}},
			wantErr: "game_mode",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := validationConfig(tc.fence)
			err := config.Validate()
			if err == nil {
				t.Fatalf("Config.Validate() error = nil, want error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Config.Validate() error = %q, want error containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestConfigValidationAcceptsSupportedVietnamMaps(t *testing.T) {
	mapNames := []string{
		"wdeva_warfare_day", "wdeva", "Vạn Tường",
		"wdevb_warfare_day", "wdevb", "Quảng Ngãi", "Quang Ngai",
		"wdevc_warfare_day", "wdevc", "Huế Outskirts", "Hue Outskirts",
		"wdevd_warfare_day", "wdevd", "Đăk Tô Airfield", "Dak To Airfield",
		"wdeve_warfare_day", "wdeve", "Cam Ranh Port",
		"wdevf_warfare_day", "wdevf", "Thanh Hòa Bridge", "Thanh Hoa Bridge",
	}

	for _, mapName := range mapNames {
		t.Run(mapName, func(t *testing.T) {
			config := validationConfig(validationFence(
				[]string{"  " + strings.ToUpper(mapName) + "  "},
				[]string{"Warfare"},
			))
			if err := config.Validate(); err != nil {
				t.Fatalf("Config.Validate() error = %v for supported map %q", err, mapName)
			}
		})
	}
}

func validationConfig(fence data.Fence) data.Config {
	return data.Config{Servers: []data.Server{{
		Host:        "192.0.2.1",
		Port:        7779,
		Password:    "secret",
		AxisFence:   []data.Fence{fence},
		AlliesFence: []data.Fence{fence},
	}}}
}

func validationFence(mapNames, gameModes []string) data.Fence {
	return data.Fence{Condition: &data.Condition{Equals: map[string][]string{
		"map_name":  mapNames,
		"game_mode": gameModes,
	}}}
}

func TestApplyServerEnvironmentExpandsCredentialFreePolicy(t *testing.T) {
	t.Setenv("HLL_HOST", "192.0.2.10")
	t.Setenv("HLL_PORT", "7779")
	t.Setenv("HLL_PASSWORD", "first-secret")
	t.Setenv("HLL_HOST_2", "192.0.2.11")
	t.Setenv("HLL_PORT_2", "7789")
	t.Setenv("HLL_PASSWORD_2", "second-secret")

	policy := data.Config{Servers: []data.Server{{
		PunishAfterSeconds: Pointer(10),
		AxisFence:          []data.Fence{validationFence([]string{"wdeva_warfare_day"}, []string{"Warfare"})},
		AlliesFence:        []data.Fence{validationFence([]string{"wdeva_warfare_day"}, []string{"Warfare"})},
	}}}

	if err := policy.ApplyServerEnvironment(); err != nil {
		t.Fatalf("ApplyServerEnvironment() error = %v", err)
	}
	if err := policy.Validate(); err != nil {
		t.Fatalf("expanded config is invalid: %v", err)
	}
	if len(policy.Servers) != 2 {
		t.Fatalf("expanded servers = %d, want 2", len(policy.Servers))
	}

	want := []struct {
		host     string
		port     int
		password string
	}{
		{host: "192.0.2.10", port: 7779, password: "first-secret"},
		{host: "192.0.2.11", port: 7789, password: "second-secret"},
	}
	for index, expected := range want {
		server := policy.Servers[index]
		if server.Host != expected.host || server.Port != expected.port || server.Password != expected.password {
			t.Errorf("Servers[%d] credentials = %s:%d/%q, want %s:%d/%q", index, server.Host, server.Port, server.Password, expected.host, expected.port, expected.password)
		}
		if server.PunishAfterSeconds == nil || *server.PunishAfterSeconds != 10 {
			t.Errorf("Servers[%d] lost policy timing", index)
		}
		if len(server.AxisFence) != 1 || len(server.AlliesFence) != 1 {
			t.Errorf("Servers[%d] lost policy fences", index)
		}
	}
}

func TestApplyServerEnvironmentRejectsCredentialsInYAML(t *testing.T) {
	t.Setenv("HLL_HOST", "192.0.2.10")
	t.Setenv("HLL_PORT", "7779")
	t.Setenv("HLL_PASSWORD", "environment-secret")

	policy := data.Config{Servers: []data.Server{{
		Host:     "192.0.2.99",
		Port:     7779,
		Password: "yaml-secret",
	}}}
	err := policy.ApplyServerEnvironment()
	if err == nil || !strings.Contains(err.Error(), "must not contain server credentials") {
		t.Fatalf("ApplyServerEnvironment() error = %v, want embedded-credentials error", err)
	}
}

func TestApplyServerEnvironmentRejectsIncompleteNumberedServer(t *testing.T) {
	t.Setenv("HLL_HOST", "192.0.2.10")
	t.Setenv("HLL_PORT", "7779")
	t.Setenv("HLL_PASSWORD", "first-secret")
	t.Setenv("HLL_HOST_2", "192.0.2.11")
	t.Setenv("HLL_PORT_2", "7789")
	t.Setenv("HLL_PASSWORD_2", "")

	policy := data.Config{Servers: []data.Server{{}}}
	err := policy.ApplyServerEnvironment()
	if err == nil || !strings.Contains(err.Error(), "HLL_PASSWORD_2") {
		t.Fatalf("ApplyServerEnvironment() error = %v, want missing HLL_PASSWORD_2 error", err)
	}
	if len(policy.Servers) != 1 {
		t.Fatal("ApplyServerEnvironment() mutated policy after rejecting credentials")
	}
}

func TestSaveOmitsCredentialFieldsFromPolicy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte("Servers:\n  - AxisFence: []\n    AlliesFence: []\n"), 0600); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	policy, err := data.NewConfig(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	if err := policy.Save(); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"Host:", "Port:", "Password:"} {
		if strings.Contains(string(contents), forbidden) {
			t.Errorf("saved credential-free policy contains %q:\n%s", forbidden, contents)
		}
	}
}

func TestApplyServerEnvironmentAppliesOperatorSettings(t *testing.T) {
	t.Setenv("HLL_HOST", "192.0.2.10")
	t.Setenv("HLL_PORT", "7779")
	t.Setenv("HLL_PASSWORD", "first-secret")
	t.Setenv("HLL_HOST_2", "192.0.2.11")
	t.Setenv("HLL_PORT_2", "7789")
	t.Setenv("HLL_PASSWORD_2", "second-secret")
	t.Setenv("GEOFENCE_LANGUAGE", "de")
	t.Setenv("GEOFENCE_PUNISH_AFTER_SECONDS", "17")
	t.Setenv("GEOFENCE_SECTOR_SIZE", "19840.5")
	t.Setenv("GEOFENCE_CENTER_OFFSET_X", "150.25")
	t.Setenv("GEOFENCE_CENTER_OFFSET_Y", "-110.5")
	t.Setenv("GEOFENCE_WARNING_MESSAGE", "Custom warning: %s seconds remain")
	t.Setenv("GEOFENCE_PUNISH_MESSAGE", "Custom punishment")

	policy := data.Config{Servers: []data.Server{{}}}
	if err := policy.ApplyServerEnvironment(); err != nil {
		t.Fatalf("ApplyServerEnvironment() error = %v", err)
	}
	if len(policy.Servers) != 2 {
		t.Fatalf("expanded servers = %d, want 2", len(policy.Servers))
	}

	for index, server := range policy.Servers {
		if server.PunishAfterSeconds == nil || *server.PunishAfterSeconds != 17 {
			t.Errorf("Servers[%d].PunishAfterSeconds = %v, want 17", index, server.PunishAfterSeconds)
		}
		if server.Grid.SectorSize != 19840.5 || server.Grid.CenterOffsetX != 150.25 || server.Grid.CenterOffsetY != -110.5 {
			t.Errorf("Servers[%d].Grid = %+v, want env-provided geometry", index, server.Grid)
		}
		if got := server.WarningMessage(); got != "Custom warning: %s seconds remain" {
			t.Errorf("Servers[%d].WarningMessage() = %q", index, got)
		}
		if got := server.PunishMessage(); got != "Custom punishment" {
			t.Errorf("Servers[%d].PunishMessage() = %q", index, got)
		}
	}
}

func TestApplyServerEnvironmentUsesLanguageDefaults(t *testing.T) {
	t.Setenv("HLL_HOST", "192.0.2.10")
	t.Setenv("HLL_PORT", "7779")
	t.Setenv("HLL_PASSWORD", "secret")
	t.Setenv("GEOFENCE_LANGUAGE", "de")

	policy := data.Config{Servers: []data.Server{{}}}
	if err := policy.ApplyServerEnvironment(); err != nil {
		t.Fatalf("ApplyServerEnvironment() error = %v", err)
	}
	server := policy.Servers[0]
	if got := server.WarningMessage(); got != "Du bist außerhalb des freigegebenen Spielbereichs. Kehre sofort zurück. Bestrafung in 10 Sekunden." {
		t.Errorf("German WarningMessage() = %q", got)
	}
	if got := server.PunishMessage(); got != "Du warst außerhalb des freigegebenen Spielbereichs." {
		t.Errorf("German PunishMessage() = %q", got)
	}
}

func TestApplyServerEnvironmentRejectsInvalidOperatorSettings(t *testing.T) {
	t.Setenv("HLL_HOST", "192.0.2.10")
	t.Setenv("HLL_PORT", "7779")
	t.Setenv("HLL_PASSWORD", "secret")

	tests := []struct {
		name    string
		key     string
		value   string
		wantErr string
	}{
		{name: "zero punish delay", key: "GEOFENCE_PUNISH_AFTER_SECONDS", value: "0", wantErr: "GEOFENCE_PUNISH_AFTER_SECONDS"},
		{name: "zero sector size", key: "GEOFENCE_SECTOR_SIZE", value: "0", wantErr: "GEOFENCE_SECTOR_SIZE"},
		{name: "invalid X offset", key: "GEOFENCE_CENTER_OFFSET_X", value: "not-a-number", wantErr: "GEOFENCE_CENTER_OFFSET_X"},
		{name: "unsupported language", key: "GEOFENCE_LANGUAGE", value: "fr", wantErr: "GEOFENCE_LANGUAGE"},
		{name: "multiline warning", key: "GEOFENCE_WARNING_MESSAGE", value: "first line\nsecond line", wantErr: "GEOFENCE_WARNING_MESSAGE"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			policy := data.Config{Servers: []data.Server{{}}}
			err := policy.ApplyServerEnvironment()
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ApplyServerEnvironment() error = %v, want error containing %q", err, tc.wantErr)
			}
		})
	}
}
