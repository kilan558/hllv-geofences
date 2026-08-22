package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/kilan558/hllv-geofences/data"
	"github.com/kilan558/hllv-geofences/maprules"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("applymap", flag.ContinueOnError)
	flags.SetOutput(stderr)
	mapName := flags.String("map", "", "map set to apply: vietnam, van-tuong, quang-ngai, hue-outskirts, dak-to-airfield, cam-ranh-port, or thanh-hoa")
	variantName := flags.String("variant", "", "geofence variant: midcap, lastcap, or midcap-advanced")
	maxPlayers := flags.Int("max-players", -1, "optional inclusive player limit for the complete Vietnam map set (0-100)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() == 0 {
		fmt.Fprintln(stderr, "usage: applymap --map <vietnam|van-tuong|quang-ngai|hue-outskirts|dak-to-airfield|cam-ranh-port|thanh-hoa> --variant <variant> <config.yml> [config.yml ...]")
		return 2
	}

	var applyMap func(*data.Config, maprules.Variant) error
	var displayName string
	switch {
	case isVietnam(*mapName):
		if *maxPlayers < -1 || *maxPlayers > 100 {
			fmt.Fprintf(stderr, "max-players must be between 0 and 100, got %d\n", *maxPlayers)
			return 2
		}
		if *maxPlayers >= 0 {
			applyMap = func(config *data.Config, variant maprules.Variant) error {
				return maprules.ApplyVietnamWithMaxPlayers(config, variant, *maxPlayers)
			}
		} else {
			applyMap = maprules.ApplyVietnam
		}
		displayName = "Vietnam map set"
	case isVanTuong(*mapName):
		applyMap = maprules.ApplyVanTuong
		displayName = "Vạn Tường"
	case isThanhHoa(*mapName):
		applyMap = maprules.ApplyThanhHoa
		displayName = "Thanh Hòa Bridge"
	case isQuangNgai(*mapName):
		applyMap = maprules.ApplyQuangNgai
		displayName = "Quảng Ngãi"
	case isHueOutskirts(*mapName):
		applyMap = maprules.ApplyHueOutskirts
		displayName = "Huế Outskirts"
	case isDakToAirfield(*mapName):
		applyMap = maprules.ApplyDakToAirfield
		displayName = "Đăk Tô Airfield"
	case isCamRanhPort(*mapName):
		applyMap = maprules.ApplyCamRanhPort
		displayName = "Cam Ranh Port"
	default:
		fmt.Fprintf(stderr, "unsupported map %q\n", *mapName)
		return 2
	}
	if *maxPlayers >= 0 && !isVietnam(*mapName) {
		fmt.Fprintln(stderr, "max-players can only be used with --map vietnam")
		return 2
	}

	variant, err := parseVariant(*variantName)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	configs := make([]*data.Config, 0, flags.NArg())
	paths := flags.Args()
	for _, path := range paths {
		config, err := data.NewConfig(path, logger)
		if err == nil {
			err = applyMap(config, variant)
		}
		if err == nil {
			err = config.ValidatePolicy()
		}
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", path, err)
			return 1
		}
		configs = append(configs, config)
	}

	for index, config := range configs {
		if err := config.Save(); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", paths[index], err)
			return 1
		}
		fmt.Fprintf(stdout, "%s: %s %s applied\n", paths[index], displayName, variant)
	}
	return 0
}

func isVietnam(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "vietnam", "all-vietnam":
		return true
	default:
		return false
	}
}

func isVanTuong(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "van-tuong", "vạn-tường", "wdeva":
		return true
	default:
		return false
	}
}

func isThanhHoa(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "thanh-hoa", "thanh-hòa", "thanh-hoa-bridge", "thanh-hòa-bridge", "wdevf":
		return true
	default:
		return false
	}
}

func isQuangNgai(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "quang-ngai", "quảng-ngãi", "wdevb":
		return true
	default:
		return false
	}
}

func isHueOutskirts(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "hue-outskirts", "huế-outskirts", "wdevc":
		return true
	default:
		return false
	}
}

func isDakToAirfield(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "dak-to-airfield", "đăk-tô-airfield", "wdevd":
		return true
	default:
		return false
	}
}

func isCamRanhPort(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "cam-ranh-port", "cam-ranh", "wdeve":
		return true
	default:
		return false
	}
}

func parseVariant(value string) (maprules.Variant, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(maprules.VariantMidcap):
		return maprules.VariantMidcap, nil
	case string(maprules.VariantLastcap):
		return maprules.VariantLastcap, nil
	case string(maprules.VariantMidcapAdvanced):
		return maprules.VariantMidcapAdvanced, nil
	default:
		return "", fmt.Errorf("unsupported variant %q", value)
	}
}
