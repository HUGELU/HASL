package main

// This pure numeric kernel is the only executable source mutated by automatic builds.
// A promoted recognition trial can generate an equivalent L1 or L2 implementation.
const compiledRecognitionMetric = "l2"

func compiledFeatureDistance(a, b []float64) float64 {
	total := 0.0
	for i := range a {
		weight := 1.0
		if a[63] > 0.5 && b[63] > 0.5 && i < 32 {
			weight = 0.02
		}
		delta := a[i] - b[i]
		total += weight * delta * delta
	}
	return total
}
