package data

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/floriansw/go-hll-rcon/rconv2"
	"gopkg.in/yaml.v3"
)

const DefaultHLLVSectorSize = 20000

type Fence struct {
	X         *string    `yaml:"X,omitempty"`
	Y         *int       `yaml:"Y,omitempty"`
	Numpads   []int      `yaml:"Numpad,omitempty"`
	Condition *Condition `yaml:"Condition,omitempty"`
}

func (f Fence) Includes(w rconv2.Grid) bool {
	if f.X != nil && w.X != *f.X {
		return false
	}
	if f.Y != nil && w.Y != *f.Y {
		return false
	}
	if len(f.Numpads) == 0 {
		return true
	}
	return slices.Contains(f.Numpads, w.Numpad)
}

func (f Fence) Matches(si *rconv2.GetSessionInfoResponse) bool {
	if f.Condition == nil {
		return true
	}
	return f.Condition.Matches(si)
}

type Condition struct {
	Equals      map[string][]string `yaml:"Equals,omitempty"`
	LessThan    map[string]int      `yaml:"LessThan,omitempty"`
	GreaterThan map[string]int      `yaml:"GreaterThan,omitempty"`
}

func (c Condition) Matches(si *rconv2.GetSessionInfoResponse) bool {
	if len(c.Equals) == 0 && len(c.LessThan) == 0 && len(c.GreaterThan) == 0 {
		return true
	}
	for k, v := range c.Equals {
		if k == "map_name" && slices.ContainsFunc(v, func(mapName string) bool {
			return strings.EqualFold(strings.TrimSpace(mapName), strings.TrimSpace(si.MapName))
		}) {
			continue
		}
		if k == "game_mode" && slices.Contains(v, si.GameMode.String()) {
			continue
		}
		return false
	}
	for k, v := range c.LessThan {
		if k == "player_count" && si.PlayerCount < v {
			continue
		}
		return false
	}
	for k, v := range c.GreaterThan {
		if k == "player_count" && si.PlayerCount > v {
			continue
		}
		return false
	}
	return true
}

type Server struct {
	Host               string       `yaml:"Host,omitempty"`
	Port               int          `yaml:"Port,omitempty"`
	Password           string       `yaml:"Password,omitempty"`
	PunishAfterSeconds *int         `yaml:"PunishAfterSeconds,omitempty"`
	Grid               GridGeometry `yaml:"Grid,omitempty"`
	AxisFence          []Fence      `yaml:"AxisFence"`
	AlliesFence        []Fence      `yaml:"AlliesFence"`
	Messages           *Messages    `yaml:"Messages,omitempty"`
}

// GridGeometry keeps world-to-grid conversion configurable while HLL:V map
// dimensions are calibrated map by map. Unreal Engine positions use centimetres.
type GridGeometry struct {
	SectorSize    float64 `yaml:"SectorSize,omitempty"`
	CenterOffsetX float64 `yaml:"CenterOffsetX,omitempty"`
	CenterOffsetY float64 `yaml:"CenterOffsetY,omitempty"`
}

func (g GridGeometry) Normalized() (GridGeometry, error) {
	if g.SectorSize == 0 {
		g.SectorSize = DefaultHLLVSectorSize
	}
	if g.SectorSize <= 0 {
		return GridGeometry{}, fmt.Errorf("Grid.SectorSize must be greater than zero, got %v", g.SectorSize)
	}
	return g, nil
}

func (s Server) PunishMessage() string {
	if punish := os.Getenv("PUNISH_MESSAGE"); punish != "" {
		return punish
	}
	if s.Messages == nil || s.Messages.Punish == nil {
		return "%s outside the play area"
	}
	return *s.Messages.Punish
}

func (s Server) WarningMessage() string {
	if warning := os.Getenv("WARNING_MESSAGE"); warning != "" {
		return warning
	}
	if s.Messages == nil || s.Messages.Warning == nil {
		return "You are outside of the designated play area! Please go back to the battlefield immediately.\n\nYou will be punished in %s"
	}
	return *s.Messages.Warning
}

func (s Server) GetWhitelist() []string {
	if whitelist := os.Getenv("WHITELIST"); whitelist != "" {
		return strings.Split(strings.TrimSpace(whitelist), ",")
	}
	return nil
}

type Messages struct {
	Warning *string `yaml:"Warning,omitempty"`
	Punish  *string `yaml:"Punish,omitempty"`
}

type Config struct {
	Servers []Server `yaml:"Servers"`
	path    string
}

func (c Config) Validate() error {
	return c.validate(true)
}

// ValidatePolicy validates map enforcement settings without requiring runtime
// connection credentials. Credentials are injected from the environment.
func (c Config) ValidatePolicy() error {
	return c.validate(false)
}

func (c Config) validate(requireCredentials bool) error {
	if len(c.Servers) == 0 {
		return errors.New("Servers must contain at least one server")
	}
	for index, server := range c.Servers {
		prefix := fmt.Sprintf("Servers[%d]", index)
		if requireCredentials {
			if strings.TrimSpace(server.Host) == "" {
				return fmt.Errorf("%s.Host must not be empty", prefix)
			}
			if server.Port < 1 || server.Port > 65535 {
				return fmt.Errorf("%s.Port must be between 1 and 65535", prefix)
			}
			if server.Password == "" {
				return fmt.Errorf("%s.Password must not be empty", prefix)
			}
		}
		if _, err := server.Grid.Normalized(); err != nil {
			return fmt.Errorf("%s: %w", prefix, err)
		}
		if server.PunishAfterSeconds != nil && *server.PunishAfterSeconds < 1 {
			return fmt.Errorf("%s.PunishAfterSeconds must be greater than zero", prefix)
		}
		if err := validateVietnamFences(prefix+".AxisFence", server.AxisFence); err != nil {
			return err
		}
		if err := validateVietnamFences(prefix+".AlliesFence", server.AlliesFence); err != nil {
			return err
		}
	}
	return nil
}

func validateVietnamFences(prefix string, fences []Fence) error {
	for index, fence := range fences {
		fencePrefix := fmt.Sprintf("%s[%d]", prefix, index)
		if fence.Condition == nil {
			return fmt.Errorf("%s.Condition is required for Vietnam-only operation", fencePrefix)
		}

		mapNames, ok := fence.Condition.Equals["map_name"]
		if !ok || len(mapNames) == 0 {
			return fmt.Errorf("%s.Condition.Equals.map_name must contain at least one supported Vietnam map", fencePrefix)
		}
		for _, mapName := range mapNames {
			if !IsSupportedVietnamMapName(mapName) {
				return fmt.Errorf("%s.Condition.Equals.map_name contains unsupported Vietnam map %q", fencePrefix, mapName)
			}
		}

		gameModes, ok := fence.Condition.Equals["game_mode"]
		if !ok || len(gameModes) == 0 {
			return fmt.Errorf("%s.Condition.Equals.game_mode must contain Warfare", fencePrefix)
		}
		for _, gameMode := range gameModes {
			if !strings.EqualFold(strings.TrimSpace(gameMode), "Warfare") {
				return fmt.Errorf("%s.Condition.Equals.game_mode contains %q; only Warfare is supported", fencePrefix, gameMode)
			}
		}
	}
	return nil
}

func (c *Config) Save() error {
	if c.path == "" {
		return errors.New("config path is empty")
	}
	existing, err := os.Stat(c.path)
	if err != nil {
		return fmt.Errorf("stat existing config: %w", err)
	}
	config, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	dir := filepath.Dir(c.path)
	tmp, err := os.CreateTemp(dir, ".geofence-config-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if owner, ok := existing.Sys().(*syscall.Stat_t); ok &&
		(owner.Uid != uint32(os.Geteuid()) || owner.Gid != uint32(os.Getegid())) {
		if err := tmp.Chown(int(owner.Uid), int(owner.Gid)); err != nil {
			tmp.Close()
			return fmt.Errorf("preserve config ownership: %w", err)
		}
	}
	if _, err := tmp.Write(config); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, c.path)
}

func NewConfig(path string, logger *slog.Logger) (*Config, error) {
	config, err := readConfig(path, logger)
	if err != nil {
		return config, err
	}

	return config, nil
}

func readConfig(path string, logger *slog.Logger) (*Config, error) {
	var config Config
	logger.Info("read-existing-config", "path", path)
	c, err := os.ReadFile(path)
	if err != nil {
		return &Config{}, err
	}
	if err := yaml.Unmarshal(c, &config); err != nil {
		return &Config{}, err
	}
	config.path = path
	return &config, nil
}
