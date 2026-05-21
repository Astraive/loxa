package storage

import (
	"math"
	"testing"

	"github.com/astraive/loxa/loxa-cortex/internal/models"
)

func TestComputeSimilarityUsesNormalizedCosine(t *testing.T) {
	a := &models.IncidentSignature{
		Shape:         "deploy->timeout",
		Symptoms:      []models.SymptomType{models.SymptomTypeTimeout},
		FeatureVector: []float64{1, 1},
	}
	b := &models.IncidentSignature{
		Shape:         "deploy->timeout",
		Symptoms:      []models.SymptomType{models.SymptomTypeTimeout},
		FeatureVector: []float64{1, 1},
	}

	gotDuckDB := computeSimilarity(a, b)
	gotPostgres := computeSimilarityPostgres(a, b)

	if math.Abs(gotDuckDB-1.0) > 0.0001 {
		t.Fatalf("expected duckdb similarity near 1, got %v", gotDuckDB)
	}
	if math.Abs(gotPostgres-1.0) > 0.0001 {
		t.Fatalf("expected postgres similarity near 1, got %v", gotPostgres)
	}
}
