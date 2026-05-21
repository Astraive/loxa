package loadgen

import (
	"fmt"
	"time"
)

type Report struct {
	Accepted int64
	Rejected int64
	Errors   int64
	Duration time.Duration
}

func (r Report) Print() {
	fmt.Printf("accepted: %d\n", r.Accepted)
	fmt.Printf("rejected: %d\n", r.Rejected)
	fmt.Printf("errors: %d\n", r.Errors)
	fmt.Printf("duration: %v\n", r.Duration)
	if r.Duration > 0 {
		fmt.Printf("throughput: %.2f events/sec\n", float64(r.Accepted)/r.Duration.Seconds())
	}
}
