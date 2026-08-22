package main

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/floriansw/go-hll-rcon/rconv2"
	"github.com/kilan558/hllv-geofences/data"
)

func TestRunAppliesVanTuongWithoutPrintingCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	contents := "Servers:\n  - Host: 192.0.2.1\n    Port: 7779\n    Password: do-not-print\n    AxisFence: []\n    AlliesFence: []\n"
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if exitCode := run([]string{
		"--map", "van-tuong",
		"--variant", "midcap",
		path,
	}, &output, &output); exitCode != 0 {
		t.Fatalf("run() exit code = %d, output: %s", exitCode, output.String())
	}
	if strings.Contains(output.String(), "do-not-print") {
		t.Fatalf("run() leaked the RCON password: %s", output.String())
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config, err := data.NewConfig(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	if err := config.Validate(); err != nil {
		t.Fatalf("updated config is invalid: %v", err)
	}
	session := &rconv2.GetSessionInfoResponse{
		MapName:     "wdeva_warfare_day",
		GameMode:    rconv2.GetSessionInfoGameModeWarfare,
		PlayerCount: 59,
	}
	matching := 0
	for _, fence := range config.Servers[0].AxisFence {
		if fence.Matches(session) {
			matching++
		}
	}
	if matching != 62 {
		t.Fatalf("matching Axis/NVA fences = %d, want 62", matching)
	}
}

func TestRunAppliesVietnamMapSetAndRemovesClassicRules(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	contents := `Servers:
  - Host: 192.0.2.1
    Port: 7779
    Password: do-not-print
    AxisFence:
      - X: A
        Y: 3
        Condition:
          Equals:
            map_name: [Carentan]
            game_mode: [Warfare]
    AlliesFence:
      - X: A
        Y: 3
        Condition:
          Equals:
            map_name: [Carentan]
            game_mode: [Warfare]
`
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if exitCode := run([]string{
		"--map", "vietnam",
		"--variant", "midcap",
		path,
	}, &output, &output); exitCode != 0 {
		t.Fatalf("run() exit code = %d, output: %s", exitCode, output.String())
	}
	if strings.Contains(output.String(), "do-not-print") {
		t.Fatalf("run() leaked the RCON password: %s", output.String())
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config, err := data.NewConfig(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	if err := config.Validate(); err != nil {
		t.Fatalf("updated config is invalid: %v", err)
	}
	for _, layer := range []string{
		"wdeva_warfare_day",
		"wdevb_warfare_day",
		"wdevc_warfare_day",
		"wdevd_warfare_day",
		"wdeve_warfare_day",
		"wdevf_warfare_day",
	} {
		session := &rconv2.GetSessionInfoResponse{
			MapName: layer, GameMode: rconv2.GetSessionInfoGameModeWarfare, PlayerCount: 59,
		}
		if got := matchingFences(config.Servers[0].AxisFence, session); got == 0 {
			t.Errorf("matching Axis/NVA fences for %s = 0, want canonical rules", layer)
		}
	}
	classic := &rconv2.GetSessionInfoResponse{
		MapName: "Carentan", GameMode: rconv2.GetSessionInfoGameModeWarfare, PlayerCount: 59,
	}
	if got := matchingFences(config.Servers[0].AxisFence, classic); got != 0 {
		t.Errorf("matching Classic-HLL fences = %d, want 0", got)
	}
}

func TestRunAppliesCustomInclusivePlayerLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	contents := "Servers:\n  - AxisFence: []\n    AlliesFence: []\n"
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if exitCode := run([]string{
		"--map", "vietnam",
		"--variant", "midcap",
		"--max-players", "42",
		path,
	}, &output, &output); exitCode != 0 {
		t.Fatalf("run() exit code = %d, output: %s", exitCode, output.String())
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config, err := data.NewConfig(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	atLimit := &rconv2.GetSessionInfoResponse{
		MapName: "wdeva_warfare_day", GameMode: rconv2.GetSessionInfoGameModeWarfare, PlayerCount: 42,
	}
	if got := matchingFences(config.Servers[0].AxisFence, atLimit); got == 0 {
		t.Fatal("matching fences at inclusive limit = 0")
	}
	afterLimit := *atLimit
	afterLimit.PlayerCount = 43
	if got := matchingFences(config.Servers[0].AxisFence, &afterLimit); got != 0 {
		t.Fatalf("matching fences after inclusive limit = %d, want 0", got)
	}
}

func TestRunAppliesThanhHoaAndPreservesVanTuong(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	contents := `Servers:
  - Host: 192.0.2.1
    Port: 7779
    Password: do-not-print
    PunishAfterSeconds: 10
    AxisFence:
      - X: A
        Y: 3
        Condition:
          Equals:
            map_name: [wdeva_warfare_day]
            game_mode: [Warfare]
    AlliesFence:
      - X: A
        Y: 3
        Condition:
          Equals:
            map_name: [wdeva_warfare_day]
            game_mode: [Warfare]
`
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if exitCode := run([]string{
		"--map", "thanh-hoa",
		"--variant", "midcap",
		path,
	}, &output, &output); exitCode != 0 {
		t.Fatalf("run() exit code = %d, output: %s", exitCode, output.String())
	}
	if strings.Contains(output.String(), "do-not-print") {
		t.Fatalf("run() leaked the RCON password: %s", output.String())
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config, err := data.NewConfig(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	server := config.Servers[0]
	if server.PunishAfterSeconds == nil || *server.PunishAfterSeconds != 10 {
		t.Fatalf("PunishAfterSeconds = %v, want 10", server.PunishAfterSeconds)
	}
	thanhHoa := &rconv2.GetSessionInfoResponse{
		MapName: "wdevf_warfare_day", GameMode: rconv2.GetSessionInfoGameModeWarfare, PlayerCount: 59,
	}
	if got := matchingFences(server.AxisFence, thanhHoa); got != 62 {
		t.Fatalf("matching Thanh Hòa Axis/NVA fences = %d, want 62", got)
	}
	vanTuong := &rconv2.GetSessionInfoResponse{
		MapName: "wdeva_warfare_day", GameMode: rconv2.GetSessionInfoGameModeWarfare, PlayerCount: 59,
	}
	if got := matchingFences(server.AxisFence, vanTuong); got != 1 {
		t.Fatalf("preserved Vạn Tường Axis/NVA fences = %d, want 1", got)
	}
}

func TestRunAppliesQuangNgaiAndPreservesExistingVietnamMaps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	contents := `Servers:
  - Host: 192.0.2.1
    Port: 7779
    Password: do-not-print
    PunishAfterSeconds: 10
    AxisFence:
      - X: A
        Y: 3
        Condition:
          Equals:
            map_name: [wdeva_warfare_day]
            game_mode: [Warfare]
      - X: C
        Y: 1
        Condition:
          Equals:
            map_name: [wdevf_warfare_day]
            game_mode: [Warfare]
    AlliesFence:
      - X: A
        Y: 3
        Condition:
          Equals:
            map_name: [wdeva_warfare_day]
            game_mode: [Warfare]
      - X: C
        Y: 1
        Condition:
          Equals:
            map_name: [wdevf_warfare_day]
            game_mode: [Warfare]
`
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if exitCode := run([]string{
		"--map", "quảng-ngãi",
		"--variant", "midcap",
		path,
	}, &output, &output); exitCode != 0 {
		t.Fatalf("run() exit code = %d, output: %s", exitCode, output.String())
	}
	if strings.Contains(output.String(), "do-not-print") {
		t.Fatalf("run() leaked the RCON password: %s", output.String())
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config, err := data.NewConfig(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	server := config.Servers[0]
	quangNgai := &rconv2.GetSessionInfoResponse{
		MapName: "wdevb_warfare_day", GameMode: rconv2.GetSessionInfoGameModeWarfare, PlayerCount: 59,
	}
	if got := matchingFences(server.AxisFence, quangNgai); got != 62 {
		t.Fatalf("matching Quảng Ngãi Axis/NVA fences = %d, want 62", got)
	}
	for _, mapName := range []string{"wdeva_warfare_day", "wdevf_warfare_day"} {
		existing := &rconv2.GetSessionInfoResponse{
			MapName: mapName, GameMode: rconv2.GetSessionInfoGameModeWarfare, PlayerCount: 59,
		}
		if got := matchingFences(server.AxisFence, existing); got != 1 {
			t.Fatalf("preserved %s Axis/NVA fences = %d, want 1", mapName, got)
		}
		if got := matchingFences(server.AlliesFence, existing); got != 1 {
			t.Fatalf("preserved %s Allies/USA fences = %d, want 1", mapName, got)
		}
	}
}

func TestIsQuangNgaiAcceptsSupportedCLIMapNames(t *testing.T) {
	for _, mapName := range []string{"quang-ngai", "quảng-ngãi", "wdevb"} {
		if !isQuangNgai(mapName) {
			t.Errorf("isQuangNgai(%q) = false, want true", mapName)
		}
	}
}

func TestRunAppliesHueOutskirtsAndPreservesExistingVietnamMaps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	contents := `Servers:
  - Host: 192.0.2.1
    Port: 7779
    Password: do-not-print
    PunishAfterSeconds: 10
    AxisFence:
      - X: A
        Y: 3
        Condition:
          Equals:
            map_name: [wdeva_warfare_day]
            game_mode: [Warfare]
      - X: C
        Y: 1
        Condition:
          Equals:
            map_name: [wdevf_warfare_day]
            game_mode: [Warfare]
      - X: J
        Y: 3
        Condition:
          Equals:
            map_name: [wdevb_warfare_day]
            game_mode: [Warfare]
    AlliesFence:
      - X: A
        Y: 3
        Condition:
          Equals:
            map_name: [wdeva_warfare_day]
            game_mode: [Warfare]
      - X: C
        Y: 1
        Condition:
          Equals:
            map_name: [wdevf_warfare_day]
            game_mode: [Warfare]
      - X: A
        Y: 3
        Condition:
          Equals:
            map_name: [wdevb_warfare_day]
            game_mode: [Warfare]
`
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if exitCode := run([]string{
		"--map", "huế-outskirts",
		"--variant", "midcap",
		path,
	}, &output, &output); exitCode != 0 {
		t.Fatalf("run() exit code = %d, output: %s", exitCode, output.String())
	}
	if strings.Contains(output.String(), "do-not-print") {
		t.Fatalf("run() leaked the RCON password: %s", output.String())
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config, err := data.NewConfig(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	server := config.Servers[0]
	hue := &rconv2.GetSessionInfoResponse{
		MapName: "wdevc_warfare_day", GameMode: rconv2.GetSessionInfoGameModeWarfare, PlayerCount: 59,
	}
	if got := matchingFences(server.AxisFence, hue); got != 62 {
		t.Fatalf("matching Huế Outskirts Axis/NVA fences = %d, want 62", got)
	}
	for _, mapName := range []string{"wdeva_warfare_day", "wdevf_warfare_day", "wdevb_warfare_day"} {
		existing := &rconv2.GetSessionInfoResponse{
			MapName: mapName, GameMode: rconv2.GetSessionInfoGameModeWarfare, PlayerCount: 59,
		}
		if got := matchingFences(server.AxisFence, existing); got != 1 {
			t.Fatalf("preserved %s Axis/NVA fences = %d, want 1", mapName, got)
		}
		if got := matchingFences(server.AlliesFence, existing); got != 1 {
			t.Fatalf("preserved %s Allies/USA fences = %d, want 1", mapName, got)
		}
	}
}

func TestIsHueOutskirtsAcceptsSupportedCLIMapNames(t *testing.T) {
	for _, mapName := range []string{"hue-outskirts", "huế-outskirts", "wdevc"} {
		if !isHueOutskirts(mapName) {
			t.Errorf("isHueOutskirts(%q) = false, want true", mapName)
		}
	}
}

func TestRunAppliesDakToAirfieldAndPreservesExistingVietnamMaps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	contents := `Servers:
  - Host: 192.0.2.1
    Port: 7779
    Password: do-not-print
    PunishAfterSeconds: 10
    AxisFence:
      - X: A
        Y: 3
        Condition:
          Equals:
            map_name: [wdeva_warfare_day, wdevb_warfare_day, wdevc_warfare_day, wdevf_warfare_day]
            game_mode: [Warfare]
    AlliesFence:
      - X: A
        Y: 3
        Condition:
          Equals:
            map_name: [wdeva_warfare_day, wdevb_warfare_day, wdevc_warfare_day, wdevf_warfare_day]
            game_mode: [Warfare]
`
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if exitCode := run([]string{
		"--map", "đăk-tô-airfield",
		"--variant", "midcap",
		path,
	}, &output, &output); exitCode != 0 {
		t.Fatalf("run() exit code = %d, output: %s", exitCode, output.String())
	}
	if strings.Contains(output.String(), "do-not-print") {
		t.Fatalf("run() leaked the RCON password: %s", output.String())
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config, err := data.NewConfig(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	server := config.Servers[0]
	dakTo := &rconv2.GetSessionInfoResponse{
		MapName: "wdevd_warfare_day", GameMode: rconv2.GetSessionInfoGameModeWarfare, PlayerCount: 59,
	}
	if got := matchingFences(server.AxisFence, dakTo); got != 62 {
		t.Fatalf("matching Đăk Tô Axis/NVA fences = %d, want 62", got)
	}
	for _, mapName := range []string{"wdeva_warfare_day", "wdevb_warfare_day", "wdevc_warfare_day", "wdevf_warfare_day"} {
		existing := &rconv2.GetSessionInfoResponse{
			MapName: mapName, GameMode: rconv2.GetSessionInfoGameModeWarfare, PlayerCount: 59,
		}
		if got := matchingFences(server.AxisFence, existing); got != 1 {
			t.Fatalf("preserved %s Axis/NVA fences = %d, want 1", mapName, got)
		}
		if got := matchingFences(server.AlliesFence, existing); got != 1 {
			t.Fatalf("preserved %s Allies/USA fences = %d, want 1", mapName, got)
		}
	}
}

func TestIsDakToAirfieldAcceptsSupportedCLIMapNames(t *testing.T) {
	for _, mapName := range []string{"dak-to-airfield", "đăk-tô-airfield", "wdevd"} {
		if !isDakToAirfield(mapName) {
			t.Errorf("isDakToAirfield(%q) = false, want true", mapName)
		}
	}
}

func TestRunAppliesCamRanhPortAndPreservesExistingVietnamMaps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	contents := `Servers:
  - Host: 192.0.2.1
    Port: 7779
    Password: do-not-print
    PunishAfterSeconds: 10
    AxisFence:
      - X: A
        Y: 3
        Condition:
          Equals:
            map_name: [wdeva_warfare_day, wdevb_warfare_day, wdevc_warfare_day, wdevd_warfare_day, wdevf_warfare_day]
            game_mode: [Warfare]
    AlliesFence:
      - X: A
        Y: 3
        Condition:
          Equals:
            map_name: [wdeva_warfare_day, wdevb_warfare_day, wdevc_warfare_day, wdevd_warfare_day, wdevf_warfare_day]
            game_mode: [Warfare]
`
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if exitCode := run([]string{
		"--map", "wdeve",
		"--variant", "midcap",
		path,
	}, &output, &output); exitCode != 0 {
		t.Fatalf("run() exit code = %d, output: %s", exitCode, output.String())
	}
	if strings.Contains(output.String(), "do-not-print") {
		t.Fatalf("run() leaked the RCON password: %s", output.String())
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config, err := data.NewConfig(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	server := config.Servers[0]
	camRanh := &rconv2.GetSessionInfoResponse{
		MapName: "wdeve_warfare_day", GameMode: rconv2.GetSessionInfoGameModeWarfare, PlayerCount: 59,
	}
	if got := matchingFences(server.AxisFence, camRanh); got != 62 {
		t.Fatalf("matching Cam Ranh Port Axis/NVA fences = %d, want 62", got)
	}
	for _, mapName := range []string{"wdeva_warfare_day", "wdevb_warfare_day", "wdevc_warfare_day", "wdevd_warfare_day", "wdevf_warfare_day"} {
		existing := &rconv2.GetSessionInfoResponse{
			MapName: mapName, GameMode: rconv2.GetSessionInfoGameModeWarfare, PlayerCount: 59,
		}
		if got := matchingFences(server.AxisFence, existing); got != 1 {
			t.Fatalf("preserved %s Axis/NVA fences = %d, want 1", mapName, got)
		}
		if got := matchingFences(server.AlliesFence, existing); got != 1 {
			t.Fatalf("preserved %s Allies/USA fences = %d, want 1", mapName, got)
		}
	}
}

func TestIsCamRanhPortAcceptsSupportedCLIMapNames(t *testing.T) {
	for _, mapName := range []string{"cam-ranh-port", "cam-ranh", "wdeve"} {
		if !isCamRanhPort(mapName) {
			t.Errorf("isCamRanhPort(%q) = false, want true", mapName)
		}
	}
}

func TestRunRejectsUnsupportedMapWithoutChangingConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	contents := []byte("Servers:\n  - Host: 192.0.2.1\n    Port: 7779\n    Password: secret\n    AxisFence: []\n    AlliesFence: []\n")
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}

	if exitCode := run([]string{"--map", "unknown", "--variant", "midcap", path}, io.Discard, io.Discard); exitCode == 0 {
		t.Fatal("run() exit code = 0, want failure for unsupported map")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, contents) {
		t.Fatal("run() changed the config after rejecting the map")
	}
}

func matchingFences(fences []data.Fence, session *rconv2.GetSessionInfoResponse) int {
	count := 0
	for _, fence := range fences {
		if fence.Matches(session) {
			count++
		}
	}
	return count
}
