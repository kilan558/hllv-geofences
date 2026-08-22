module github.com/kilan558/hllv-geofences

go 1.26

toolchain go1.26.6

// HLL: Vietnam support in this pinned upstream revision is still work in
// progress. The audited local copy contains the required compatibility fixes
// for generated types, game-specific methods, position precision, and broken
// connection retirement until those changes are available upstream.
replace github.com/floriansw/go-hll-rcon => ./third_party/go-hll-rcon

require (
	github.com/floriansw/go-hll-rcon v0.0.0-20260813222811-057e2ffe848e
	github.com/joho/godotenv v1.5.1
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.36.2
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
)
