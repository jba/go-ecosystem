// Package progress is for tracking the progress of long-running
// operations.
package progress

import (
	"fmt"
	"log"
	"sync/atomic"
	"time"
)

// Info holds information about the progress of some activity.
type Info struct {
	Total      int           // the total number of work units to do
	Done       int           // how much of the total has been done
	DoneRecent int           // how much has been done since the last call to report
	Rate       float64       // the rate at which work is being done, in work units per second
	RateRecent float64       // the rate over the last interval
	ETA        time.Duration // the estimated time remaining to complete the work
}

func (i Info) String() string {
	if i.Total < 0 {
		return fmt.Sprintf("%d/? %.1f/s  %.1f/s recent", i.Done, i.Rate, i.RateRecent)
	}
	return fmt.Sprintf("%d/%d (%2d%%)  %.1f/s  %.1f/s recent  ETA %s",
		i.Done, i.Total, i.Done*100/i.Total, i.Rate, i.RateRecent, i.ETA)
}

// A Tracker tracks progress.
// The nil tracker does nothing.
type Tracker struct {
	done       atomic.Int64
	doneRecent atomic.Int64
	stopped    bool
	stopc      chan struct{}
}

// Did marks n units of work as done.
func (t *Tracker) Did(n int) {
	if t != nil {
		t.done.Add(int64(n))
		t.doneRecent.Add(int64(n))
	}
}

// Stop ends tracking. Call it to free resources allocated by [Start].
// Stop can be called multiple times.
func (t *Tracker) Stop() {
	if t != nil && !t.stopped {
		close(t.stopc)
		t.stopped = true
	}
}

// Start starts tracking progress.
// Total is the total amount of work to do.
// If total is negative, only the amount of work done is known, not information about completion.
// The report function is called at the given interval with information about progress.
// If nil, a default report function is used.
func Start(total int, interval time.Duration, report func(Info)) *Tracker {
	if report == nil {
		report = Log("progress")
	}
	start := time.Now()
	ticker := time.NewTicker(interval)
	t := &Tracker{stopc: make(chan struct{})}

	go func() {
		lastReport := start
		for {
			select {
			case <-ticker.C:
				info := Info{Total: total}
				info.Done = int(t.done.Load())
				info.DoneRecent = int(t.doneRecent.Load())
				info.Rate = float64(info.Done) / time.Since(start).Seconds()
				info.RateRecent = float64(info.DoneRecent) / time.Since(lastReport).Seconds()
				if total >= 0 {
					info.ETA = time.Duration(float64(total-info.Done)/info.Rate) * time.Second
				}
				report(info)
				lastReport = time.Now()
				t.doneRecent.Store(0)
			case <-t.stopc:
				return
			}
		}
	}()

	return t
}

// Log uses the default [log.Logger] to print an [Info].
// It can be passed as the report function to [Start].
func Log(prefix string) func(Info) {
	return func(i Info) {
		log.Printf("%s: %s", prefix, i)
	}
}
