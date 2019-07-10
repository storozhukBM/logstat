package stat

type Record struct {
	UnixTime     int64
	Section      string
	StatusCode   int32
	ResponseSize int64
}

/*
Cycle report with immutable public interface if shared by value
*/
type Report struct {
	CycleDurationInSeconds int64
	CycleOffset            int64
	CycleStartUnixTime     int64

	TotalRequests            uint64
	TotalResponseSizeInBytes uint64
	requestsPerSection       map[string]uint64
	requestsPerStatusCode    map[int32]uint64
}

func BuildReport(requestsPerSection map[string]uint64, requestsPerStatusCode map[int32]uint64) Report {
	result := Report{
		requestsPerSection:    make(map[string]uint64, len(requestsPerSection)),
		requestsPerStatusCode: make(map[int32]uint64, len(requestsPerStatusCode)),
	}
	for section, requests := range requestsPerSection {
		result.requestsPerSection[section] = requests
	}
	for code, requests := range requestsPerStatusCode {
		result.requestsPerStatusCode[code] = requests
	}
	return result
}

func (c Report) IterRequestsPerSection(iteration func(section string, requests uint64)) {
	for section, requests := range c.requestsPerSection {
		iteration(section, requests)
	}
}

func (c Report) GetRequestsPerSection(section string) uint64 {
	return c.requestsPerSection[section]
}

func (c Report) IterRequestsPerStatusCode(iteration func(code int32, requests uint64)) {
	for code, requests := range c.requestsPerStatusCode {
		iteration(code, requests)
	}
}

func (c Report) GetRequestsPerStatusCode(code int32) uint64 {
	return c.requestsPerStatusCode[code]
}
