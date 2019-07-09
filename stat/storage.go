package stat

import "github.com/storozhukBM/logstat/common/log"

type StatsStorage struct {
	count int
}

func NewStatsStorage() (*StatsStorage, error) {
	return &StatsStorage{}, nil
}

func (s *StatsStorage) Store(r Record) {
	s.count++
	if s.count%10000 == 0 {
		log.Debug("record: %+v", r)
	}
}
