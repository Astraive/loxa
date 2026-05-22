module github.com/astraive/loxa/sdks/go/bench

go 1.25.0

require (
	github.com/astraive/loxa/sdks/go v0.0.0
	github.com/astraive/loxa/sdks/go/src/middleware v0.0.0
)

require (
	github.com/astraive/loxa/spec v0.0.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.66.1 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	go.opentelemetry.io/otel v1.43.0 // indirect
	go.opentelemetry.io/otel/trace v1.43.0 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	golang.org/x/sys v0.43.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace github.com/astraive/loxa/sdks/go => ../

replace github.com/astraive/loxa/sdks/go/src/middleware => ../src/middleware

replace github.com/astraive/loxa/spec => ../../../spec
