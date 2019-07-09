package w3c

import (
	"bytes"
	"fmt"
	"github.com/storozhukBM/logstat/common/log"
	"github.com/storozhukBM/logstat/common/pnc"
	"github.com/storozhukBM/logstat/stat"
	"hash"
	"hash/fnv"
	"strconv"
	"time"
	"unsafe"
)

type lineProvider interface {
	Output() <-chan []byte
}

type store interface {
	Store(r stat.Record)
}

type W3CLogToStoreAdapter struct {
	lineProvider lineProvider
	store        store

	sectionInternCacheSize  int
	stringInternHash        hash.Hash32
	sectionPartsInternCache map[uint32]string
}

func NewW3CLogToStoreAdapter(lineProvider lineProvider, store store, sectionInternCacheSize uint) (*W3CLogToStoreAdapter, error) {
	if lineProvider == nil {
		return nil, fmt.Errorf("lineProvider can't be nil")
	}
	if store == nil {
		return nil, fmt.Errorf("store can't be nil")
	}
	if lineProvider.Output() == nil {
		return nil, fmt.Errorf("lineProvider output can't be nil")
	}
	result := &W3CLogToStoreAdapter{
		lineProvider: lineProvider,
		store:        store,

		sectionInternCacheSize:  int(sectionInternCacheSize),
		stringInternHash:        fnv.New32a(),
		sectionPartsInternCache: make(map[uint32]string),
	}
	go result.run()
	return result, nil
}

func (a *W3CLogToStoreAdapter) run() {
	input := a.lineProvider.Output()
	for {
		_, opened := <-input
		if !opened {
			return
		}
		a.cycle(input)
	}
}
func (a *W3CLogToStoreAdapter) cycle(input <-chan []byte) {
	defer pnc.PanicHandle()
	for line := range input {
		record, parseErr := a.parse(line)
		if parseErr != nil {
			log.WithError(parseErr, "can't parse line")
			continue
		}
		a.store.Store(record)
	}
}

func (a *W3CLogToStoreAdapter) parse(line []byte) (stat.Record, error) {
	timePartStart, prefixErr := a.skipPrefix(line)
	if prefixErr != nil {
		return stat.Record{}, fmt.Errorf("can't skip prefix: %v", prefixErr)
	}
	timePartEnd, unixTime, timeParsingErr := a.findAndParseTimePart(line, timePartStart)
	if timeParsingErr != nil {
		return stat.Record{}, fmt.Errorf("can't parse time: %v", timeParsingErr)
	}
	sectionPartEnd, sectionPartStr, sectionParsingErr := a.findAndParseSectionPart(line, timePartEnd)
	if sectionParsingErr != nil {
		return stat.Record{}, fmt.Errorf("can't parse section: %v", sectionParsingErr)
	}
	statusCodePartEnd, statusCode, statusCodeParsingErr := a.findAndParseStatusCodePart(line, sectionPartEnd)
	if statusCodeParsingErr != nil {
		return stat.Record{}, fmt.Errorf("can't parse status code: %v", statusCodeParsingErr)
	}
	bodySize, bodySizeParsingErr := a.findAndParseBodySize(line, statusCodePartEnd)
	if bodySizeParsingErr != nil {
		return stat.Record{}, fmt.Errorf("can't parse body size: %v", bodySizeParsingErr)
	}

	return stat.Record{
		UnixTime:     unixTime,
		Section:      sectionPartStr,
		StatusCode:   statusCode,
		ResponseSize: bodySize,
	}, nil
}

func (a *W3CLogToStoreAdapter) skipPrefix(line []byte) (int, error) {
	timePartStart := a.skip(line, ' ', 3)
	if timePartStart == -1 {
		return 0, fmt.Errorf("enexpected format of line. not enough spaces in prefix")
	}
	return timePartStart, nil
}

func (a *W3CLogToStoreAdapter) findAndParseTimePart(line []byte, timePartStart int) (int, int64, error) {
	timePartStart += 1 // skip `[` from time part
	if timePartStart >= len(line) {
		return 0, 0, fmt.Errorf("enexpected format of line. missed `[` in time part")
	}
	timePartEnd := bytes.IndexByte(line[timePartStart:], ']')
	if timePartEnd == -1 {
		return 0, 0, fmt.Errorf("enexpected format of line. can't parse time part end")
	}
	timePartEnd = timePartStart + timePartEnd // timePartEnd is relative to timePartStart
	timePart := line[timePartStart:timePartEnd]

	unixTime, timeParseErr := a.parseTimePart(timePart)
	if timeParseErr != nil {
		return 0, 0, timeParseErr
	}
	return timePartEnd, unixTime, nil
}

func (a *W3CLogToStoreAdapter) findAndParseSectionPart(line []byte, timePartEnd int) (int, string, error) {
	sectionPartStart := bytes.IndexByte(line[timePartEnd:], '/')
	if sectionPartStart == -1 {
		return 0, "", fmt.Errorf("enexpected format of line. can't parse section part")
	}
	sectionPartStart = timePartEnd + sectionPartStart // sectionPartStart is relative to timePartEnd

	sectionPartEnd := bytes.IndexByte(line[sectionPartStart:], ' ')
	if sectionPartEnd == -1 {
		return 0, "", fmt.Errorf("enexpected format of line. can't parse section part end")
	}
	sectionPartEnd = sectionPartStart + sectionPartEnd // sectionPartEnd is relative to sectionPartStart

	sectionPart := line[sectionPartStart:sectionPartEnd]
	subSectionEnd := bytes.IndexByte(sectionPart[1:], '/')
	if subSectionEnd != -1 {
		sectionPart = sectionPart[0 : subSectionEnd+1]
	}
	sectionPartStr := a.internSectionString(sectionPart)
	return sectionPartEnd, sectionPartStr, nil
}

func (a *W3CLogToStoreAdapter) findAndParseStatusCodePart(line []byte, sectionPartEnd int) (int, int32, error) {
	sectionPartEnd = sectionPartEnd + 1 // skip ` ` from time protocol part
	if sectionPartEnd >= len(line) {
		return 0, 0, fmt.Errorf("enexpected format of line. missed ` ` in protocol part")
	}
	statusCodePartStart := bytes.IndexByte(line[sectionPartEnd:], ' ')
	if statusCodePartStart == -1 {
		return 0, 0, fmt.Errorf("enexpected format of line. can't parse status code start")
	}
	statusCodePartStart = sectionPartEnd + statusCodePartStart // statusCodePartStart is relative to sectionPartEnd

	statusCodePartStart = statusCodePartStart + 1 // skip ` ` from time protocol part
	statusCodePartEnd := statusCodePartStart + 3  // status code should take 3 chars
	if statusCodePartEnd >= len(line) {
		return 0, 0, fmt.Errorf("enexpected format of line. staus code part is cropped")
	}
	statusCodePart := line[statusCodePartStart:statusCodePartEnd]
	statusCode, statusCodeParsingErr := a.parseInt64(statusCodePart)
	if statusCodeParsingErr != nil {
		return 0, 0, statusCodeParsingErr
	}
	return statusCodePartEnd, int32(statusCode), nil
}

func (a *W3CLogToStoreAdapter) findAndParseBodySize(line []byte, statusCodePartEnd int) (int64, error) {
	bodySizePartStart := statusCodePartEnd + 1 // skip ` ` from time body size part
	if bodySizePartStart >= len(line) {
		return 0, fmt.Errorf("enexpected format of line. missed ` ` in body size part")
	}
	bodySizePart := line[bodySizePartStart:]
	bodySize, bodySizeParsingErr := a.parseInt64(bodySizePart)
	if bodySizeParsingErr != nil {
		return 0, bodySizeParsingErr
	}
	return bodySize, nil
}

func (a *W3CLogToStoreAdapter) skip(line []byte, separator byte, n int) int {
	count := n
	target := line
	for count > 0 {
		idx := bytes.IndexByte(target, separator)
		if idx == -1 || idx+1 == len(target) {
			return -1
		}
		target = target[idx+1:]
		count--
	}
	return len(line) - len(target)
}

/*
This parser potentially can be slow (due to generalized layout).
In future can be replaced with specialized one.
*/
func (a *W3CLogToStoreAdapter) parseTimePart(timePart []byte) (int64, error) {
	timePartStr := *(*string)(unsafe.Pointer(&timePart)) // bytes to string without potential allocation
	t, parsingErr := time.Parse("02/Jan/2006:15:04:05 -0700", timePartStr)
	if parsingErr != nil {
		return 0, parsingErr
	}
	return t.Unix(), nil
}

func (a *W3CLogToStoreAdapter) parseInt64(intPart []byte) (int64, error) {
	str := *(*string)(unsafe.Pointer(&intPart)) // bytes to string without potential allocation
	result, parseErr := strconv.ParseInt(str, 10, 64)
	if parseErr != nil {
		return 0, fmt.Errorf("can't parse int from `%s`: %v", str, parseErr)
	}
	return result, nil
}

func (a *W3CLogToStoreAdapter) internSectionString(sectionPart []byte) string {
	if a.sectionInternCacheSize == 0 {
		return string(sectionPart)
	}
	if len(a.sectionPartsInternCache) > a.sectionInternCacheSize {
		a.sectionPartsInternCache = nil
	}

	sectionPartStr := *(*string)(unsafe.Pointer(&sectionPart)) // bytes to string without potential allocation
	a.stringInternHash.Reset()
	_, _ = a.stringInternHash.Write(sectionPart)
	sectionHash := a.stringInternHash.Sum32()
	partFromCache, ok := a.sectionPartsInternCache[sectionHash]
	if ok && sectionPartStr == partFromCache {
		return partFromCache
	}
	sectionString := string(sectionPart)
	a.sectionPartsInternCache[sectionHash] = sectionString
	return sectionString
}
