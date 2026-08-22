package data

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

type serverCredentials struct {
	host     string
	port     int
	password string
}

// ApplyServerEnvironment expands one credential-free YAML policy template
// into one runtime server per HLL_HOST/HLL_PORT/HLL_PASSWORD tuple. Additional
// servers use contiguous numeric suffixes starting with _2.
func (c *Config) ApplyServerEnvironment() error {
	if c == nil {
		return fmt.Errorf("config must not be nil")
	}
	if len(c.Servers) != 1 {
		return fmt.Errorf("credential-free policy must contain exactly one Servers template, got %d", len(c.Servers))
	}
	template := c.Servers[0]
	if template.Host != "" || template.Port != 0 || template.Password != "" {
		return fmt.Errorf("credential-free policy must not contain server credentials; use HLL_HOST, HLL_PORT, and HLL_PASSWORD")
	}
	template, err := applyOperatorEnvironment(template)
	if err != nil {
		return err
	}
	policy := Config{Servers: []Server{template}}
	if err := policy.ValidatePolicy(); err != nil {
		return err
	}

	indexes, err := environmentServerIndexes()
	if err != nil {
		return err
	}
	credentials := make([]serverCredentials, 0, len(indexes))
	for _, index := range indexes {
		credential, err := readServerCredentials(index)
		if err != nil {
			return err
		}
		credentials = append(credentials, credential)
	}

	servers := make([]Server, 0, len(credentials))
	for _, credential := range credentials {
		server := template
		server.Host = credential.host
		server.Port = credential.port
		server.Password = credential.password
		servers = append(servers, server)
	}
	c.Servers = servers
	return c.Validate()
}

func applyOperatorEnvironment(server Server) (Server, error) {
	punishAfter := 10
	if server.PunishAfterSeconds != nil {
		punishAfter = *server.PunishAfterSeconds
	}
	if value, ok := os.LookupEnv("GEOFENCE_PUNISH_AFTER_SECONDS"); ok {
		parsed, err := parseBoundedInt("GEOFENCE_PUNISH_AFTER_SECONDS", value, 1, 3600)
		if err != nil {
			return Server{}, err
		}
		punishAfter = parsed
	}
	server.PunishAfterSeconds = &punishAfter

	sectorSize := server.Grid.SectorSize
	if sectorSize == 0 {
		sectorSize = DefaultHLLVSectorSize
	}
	if value, ok := os.LookupEnv("GEOFENCE_SECTOR_SIZE"); ok {
		parsed, err := parseFiniteFloat("GEOFENCE_SECTOR_SIZE", value)
		if err != nil {
			return Server{}, err
		}
		if parsed <= 0 {
			return Server{}, fmt.Errorf("GEOFENCE_SECTOR_SIZE must be greater than zero")
		}
		sectorSize = parsed
	}
	server.Grid.SectorSize = sectorSize

	offsets := []struct {
		key    string
		target *float64
	}{
		{key: "GEOFENCE_CENTER_OFFSET_X", target: &server.Grid.CenterOffsetX},
		{key: "GEOFENCE_CENTER_OFFSET_Y", target: &server.Grid.CenterOffsetY},
	}
	for _, offset := range offsets {
		if value, ok := os.LookupEnv(offset.key); ok {
			parsed, err := parseFiniteFloat(offset.key, value)
			if err != nil {
				return Server{}, err
			}
			*offset.target = parsed
		}
	}

	language := "en"
	if value, ok := os.LookupEnv("GEOFENCE_LANGUAGE"); ok {
		language = strings.ToLower(strings.TrimSpace(value))
	}
	var warningMessage, punishMessage string
	switch language {
	case "en":
		warningMessage = fmt.Sprintf("You are outside the designated play area. Return to the battlefield immediately. Punishment in %d seconds.", punishAfter)
		punishMessage = "You remained outside the designated play area."
	case "de":
		warningMessage = fmt.Sprintf("Du bist außerhalb des freigegebenen Spielbereichs. Kehre sofort zurück. Bestrafung in %d Sekunden.", punishAfter)
		punishMessage = "Du warst außerhalb des freigegebenen Spielbereichs."
	default:
		return Server{}, fmt.Errorf("GEOFENCE_LANGUAGE must be en or de, got %q", language)
	}

	var err error
	warningMessage, err = configuredMessage("GEOFENCE_WARNING_MESSAGE", warningMessage)
	if err != nil {
		return Server{}, err
	}
	punishMessage, err = configuredMessage("GEOFENCE_PUNISH_MESSAGE", punishMessage)
	if err != nil {
		return Server{}, err
	}
	server.Messages = &Messages{Warning: &warningMessage, Punish: &punishMessage}
	return server, nil
}

func parseBoundedInt(key, value string, minimum, maximum int) (int, error) {
	value = strings.TrimSpace(value)
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < minimum || parsed > maximum {
		return 0, fmt.Errorf("%s must be an integer between %d and %d", key, minimum, maximum)
	}
	return parsed, nil
}

func parseFiniteFloat(key, value string) (float64, error) {
	value = strings.TrimSpace(value)
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, fmt.Errorf("%s must be a finite number", key)
	}
	return parsed, nil
}

func configuredMessage(key, fallback string) (string, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s must not be empty", key)
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("%s must not contain line breaks", key)
	}
	return value, nil
}

func environmentServerIndexes() ([]int, error) {
	indexes := map[int]struct{}{1: {}}
	prefixes := []string{"HLL_HOST_", "HLL_PORT_", "HLL_PASSWORD_"}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		for _, prefix := range prefixes {
			if !strings.HasPrefix(key, prefix) {
				continue
			}
			suffix := strings.TrimPrefix(key, prefix)
			index, err := strconv.Atoi(suffix)
			if err != nil || index < 2 {
				return nil, fmt.Errorf("%s has an invalid server suffix; use _2, _3, and so on", key)
			}
			indexes[index] = struct{}{}
		}
	}

	result := make([]int, 0, len(indexes))
	for index := range indexes {
		result = append(result, index)
	}
	sort.Ints(result)
	for offset, index := range result {
		expected := offset + 1
		if index != expected {
			return nil, fmt.Errorf("server numbering must be contiguous; expected server %d, found server %d", expected, index)
		}
	}
	return result, nil
}

func readServerCredentials(index int) (serverCredentials, error) {
	suffix := ""
	if index > 1 {
		suffix = "_" + strconv.Itoa(index)
	}

	hostKey := "HLL_HOST" + suffix
	portKey := "HLL_PORT" + suffix
	passwordKey := "HLL_PASSWORD" + suffix
	host := strings.TrimSpace(os.Getenv(hostKey))
	portValue := strings.TrimSpace(os.Getenv(portKey))
	password := os.Getenv(passwordKey)
	for key, value := range map[string]string{
		hostKey:     host,
		portKey:     portValue,
		passwordKey: password,
	} {
		if strings.TrimSpace(value) == "" {
			return serverCredentials{}, fmt.Errorf("%s must not be empty", key)
		}
		if strings.ContainsAny(value, "\r\n") {
			return serverCredentials{}, fmt.Errorf("%s must not contain line breaks", key)
		}
	}

	port, err := strconv.Atoi(portValue)
	if err != nil || port < 1 || port > 65535 {
		return serverCredentials{}, fmt.Errorf("%s must be an integer between 1 and 65535", portKey)
	}
	return serverCredentials{host: host, port: port, password: password}, nil
}
