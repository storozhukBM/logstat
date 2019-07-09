package stat

import "github.com/storozhukBM/logstat/common/log"

type StatsStorage struct {
}

func NewStatsStorage() (*StatsStorage, error) {
	return &StatsStorage{}, nil
}

func (*StatsStorage) Store(r Record) {
	log.Debug("record: %+v", r)
}
