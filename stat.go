package recache

import (
	"sync/atomic"
	"time"
)

const statInterval = time.Minute

type stat struct {
	name    string
	total   uint64
	hit     uint64
	miss    uint64
	dbFails uint64
	log     logger
}

func NewStat(name string) *stat {
	ret := &stat{
		name: name,
	}

	go func() {
		ticker := time.NewTicker(statInterval)
		defer ticker.Stop()

		ret.statLoop(ticker)
	}()

	return ret
}

func (s *stat) incrementTotal() {
	atomic.AddUint64(&s.total, 1)
}

func (s *stat) incrementHit() {
	atomic.AddUint64(&s.hit, 1)
}

func (s *stat) incrementMiss() {
	atomic.AddUint64(&s.miss, 1)
}

func (s *stat) incrementDbFails() {
	atomic.AddUint64(&s.dbFails, 1)
}

func (s *stat) statLoop(ticker *time.Ticker) {
	for range ticker.C {
		total := atomic.SwapUint64(&s.total, 0)
		if total == 0 {
			continue
		}

		hit := atomic.SwapUint64(&s.hit, 0)
		percent := 100 * float32(hit) / float32(total)
		miss := atomic.SwapUint64(&s.miss, 0)
		dbf := atomic.SwapUint64(&s.dbFails, 0)
		s.log.Infof("dbcache(%s) - qpm: %d, hit_ratio: %.1f%%, hit: %d, miss: %d, db_fails: %d",
			s.name, total, percent, hit, miss, dbf)
	}
}
