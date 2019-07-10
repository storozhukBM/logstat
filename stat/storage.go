package stat

import (
	"fmt"
)

type Storage struct {
	cycleDurationInSeconds int64

	currentCycle   *Report
	prevCyclesRing chan Report
}

func NewStorage(cycleDurationInSeconds uint64, prevCyclesRingSize uint) (*Storage, error) {
	if cycleDurationInSeconds < 1 {
		return nil, fmt.Errorf("CycleDurationInSeconds should be at least 1")
	}
	if prevCyclesRingSize < 1 {
		return nil, fmt.Errorf("prevCyclesRingSize should be at least 1")
	}
	return &Storage{
		cycleDurationInSeconds: int64(cycleDurationInSeconds),
		currentCycle:           nil,
		prevCyclesRing:         make(chan Report, prevCyclesRingSize),
	}, nil
}

func (s *Storage) Store(r Record) {
	recordOffset := r.UnixTime / s.cycleDurationInSeconds
	s.currentCycle = s.tryRotateCurrentCycle(recordOffset)

	s.currentCycle.TotalRequests++
	s.currentCycle.TotalResponseSizeInBytes += uint64(r.ResponseSize)
	s.currentCycle.requestsPerSection[r.Section]++
	s.currentCycle.requestsPerStatusCode[r.StatusCode]++
}

func (s *Storage) Reports() <-chan Report {
	return s.prevCyclesRing
}

func (s *Storage) tryRotateCurrentCycle(recordOffset int64) *Report {
	if s.currentCycle == nil {
		return &Report{
			CycleDurationInSeconds:   s.cycleDurationInSeconds,
			CycleOffset:              recordOffset,
			CycleStartUnixTime:       recordOffset * s.cycleDurationInSeconds,
			TotalRequests:            0,
			TotalResponseSizeInBytes: 0,
			requestsPerSection:       make(map[string]uint64),
			requestsPerStatusCode:    make(map[int32]uint64),
		}
	}
	if s.currentCycle.CycleOffset == recordOffset {
		return s.currentCycle
	}

	oldCycle := s.currentCycle
	newCycle := &Report{
		CycleDurationInSeconds:   s.cycleDurationInSeconds,
		CycleOffset:              recordOffset,
		CycleStartUnixTime:       recordOffset * s.cycleDurationInSeconds,
		TotalRequests:            0,
		TotalResponseSizeInBytes: 0,
		requestsPerSection:       make(map[string]uint64, len(oldCycle.requestsPerSection)),
		requestsPerStatusCode:    make(map[int32]uint64, len(oldCycle.requestsPerStatusCode)),
	}

	select {
	case s.prevCyclesRing <- *oldCycle:
	default:
		<-s.prevCyclesRing
		s.prevCyclesRing <- *oldCycle
	}

	return newCycle
}
