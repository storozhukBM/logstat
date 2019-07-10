package alert

type TrafficAlert struct {
	AlertID                  uint64
	Resolved                 bool
	MaxAllowedRequests       uint64
	ObservedInWindowRequests uint64
	WindowStartUnixTime      int64
	WindowEndUnixTime        int64
}
