package rolling

import "math"

// EWStats maintains exponentially weighted mean/variance for streaming z-scores.
type EWStats struct {
    Mean  float64
    Var   float64
    Count int
    Alpha float64
}

// Update ingests a new observation.
func (s *EWStats) Update(x float64) {
    s.Count++
    if s.Count == 1 {
        s.Mean = x
        s.Var = 0
        return
    }
    d := x - s.Mean
    s.Mean += s.Alpha * d
    s.Var = (1 - s.Alpha) * (s.Var + s.Alpha*d*d)
}

// Z returns the normalized z-score for x based on current state.
func (s *EWStats) Z(x float64) float64 {
    sd := math.Sqrt(math.Max(s.Var, 1e-12))
    if sd == 0 {
        return 0
    }
    return (x - s.Mean) / sd
}
