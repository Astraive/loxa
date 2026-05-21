module github.com/astraive/loxa-go/examples/httpbatch-to-collector

go 1.25.0

require github.com/astraive/loxa-go v0.0.0

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	go.opentelemetry.io/otel v1.43.0 // indirect
	go.opentelemetry.io/otel/trace v1.43.0 // indirect
)

replace github.com/astraive/loxa-go => ../..
