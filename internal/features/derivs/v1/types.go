package derivsv1

import "time"

// ValuePoint captures a timestamped scalar observation.
type ValuePoint struct {
    Timestamp time.Time
    Value     float64
}

// BasisPoint stores mark/index pair for a venue at a timestamp.
type BasisPoint struct {
    Timestamp time.Time
    Mark      float64
    Index     float64
}
