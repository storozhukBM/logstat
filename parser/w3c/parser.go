package w3c

import (
	"bytes"
	"fmt"
	"github.com/storozhukBM/logstat/stat"
	"hash"
	"hash/fnv"
	"time"
	"unsafe"
)

/*
A component used to parse one line of log in w3c format to storage req record.
This parser should work reasonably fast due to the extensive use of `bytes.IndexByte` method,
that is accelerated using vectorized instructions.
Under load in hot path this parser should work with almost zero allocations.

Responsibilities:
	- parse logline to req record
	- copy required parts of line bytes to separate storage or structure,
	so line bytes can be recycled and reused afterward

Attention:
	- sections string are cached in `sectionPartsInternCache` to avoid allocations,
	but we enforce certain cache size as protection from memory leaks. This should work OK,
	because in typical server log there is a fixed amount of sections

Future:
	- this implementation require fuzz testing in future
	- it can be further optimized by ensuring the inlinement of certain functions and
	elimination of unnecessary bounds checks. Use `go build -gcflags '-m -m -d=ssa/check_bce/debug=1' ./...` for details.
*/
type LineToStoreRecordParser struct {
	sectionInternCacheSize  int
	stringInternHash        hash.Hash32
	sectionPartsInternCache map[uint32]string

	lastFullyParsedTimePart                  []byte
	lastFullyParsedTimeStartOfTheDayUnixTime int64
}

func NewLineToStoreRecordParser(sectionInternCacheSize uint) (*LineToStoreRecordParser, error) {
	result := &LineToStoreRecordParser{
		sectionInternCacheSize:  int(sectionInternCacheSize),
		stringInternHash:        fnv.New32a(),
		sectionPartsInternCache: make(map[uint32]string),
	}
	return result, nil
}

func (p *LineToStoreRecordParser) Parse(line []byte) (stat.Record, error) {
	timePartStart, prefixErr := p.skipPrefix(line)
	if prefixErr != nil {
		return stat.Record{}, fmt.Errorf("can't skip prefix: %v", prefixErr)
	}
	timePartEnd, unixTime, timeParsingErr := p.findAndParseTimePart(line, timePartStart)
	if timeParsingErr != nil {
		return stat.Record{}, fmt.Errorf("can't parse time: %v", timeParsingErr)
	}
	sectionPartEnd, sectionPartStr, sectionParsingErr := p.findAndParseSectionPart(line, timePartEnd)
	if sectionParsingErr != nil {
		return stat.Record{}, fmt.Errorf("can't parse section: %v", sectionParsingErr)
	}
	statusCodePartEnd, statusCode, statusCodeParsingErr := p.findAndParseStatusCodePart(line, sectionPartEnd)
	if statusCodeParsingErr != nil {
		return stat.Record{}, fmt.Errorf("can't parse status code: %v", statusCodeParsingErr)
	}
	bodySize, bodySizeParsingErr := p.findAndParseBodySize(line, statusCodePartEnd)
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

func (p *LineToStoreRecordParser) skipPrefix(line []byte) (int, error) {
	timePartStart := p.skip(line, ' ', 3)
	if timePartStart == -1 {
		return 0, fmt.Errorf("enexpected format of line. not enough spaces in prefix")
	}
	return timePartStart, nil
}

func (p *LineToStoreRecordParser) findAndParseTimePart(line []byte, timePartStart int) (int, int64, error) {
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

	unixTime, timeParseErr := p.parseTimePart(timePart)
	if timeParseErr != nil {
		return 0, 0, timeParseErr
	}
	return timePartEnd, unixTime, nil
}

func (p *LineToStoreRecordParser) findAndParseSectionPart(line []byte, timePartEnd int) (int, string, error) {
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
	sectionPartStr := p.internSectionString(sectionPart)
	return sectionPartEnd, sectionPartStr, nil
}

func (p *LineToStoreRecordParser) findAndParseStatusCodePart(line []byte, sectionPartEnd int) (int, int32, error) {
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
	statusCode, statusCodeParsingErr := p.parseInt64(statusCodePart)
	if statusCodeParsingErr != nil {
		return 0, 0, statusCodeParsingErr
	}
	return statusCodePartEnd, int32(statusCode), nil
}

func (p *LineToStoreRecordParser) findAndParseBodySize(line []byte, statusCodePartEnd int) (int64, error) {
	bodySizePartStart := statusCodePartEnd + 1 // skip ` ` from time body size part
	if bodySizePartStart >= len(line) {
		return 0, fmt.Errorf("enexpected format of line. missed ` ` in body size part")
	}
	bodySizePart := line[bodySizePartStart:]
	bodySize, bodySizeParsingErr := p.parseInt64(bodySizePart)
	if bodySizeParsingErr != nil {
		return 0, bodySizeParsingErr
	}
	return bodySize, nil
}

func (p *LineToStoreRecordParser) skip(line []byte, separator byte, n int) int {
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
Time parser with cache to avoid parsing date and timezone.
*/
func (p *LineToStoreRecordParser) parseTimePart(timePart []byte) (int64, error) {
	if p.lastFullyParsedTimePart == nil {
		return p.regularTimeParse(timePart)
	}

	{
		cacheZonePart := p.lastFullyParsedTimePart[len(p.lastFullyParsedTimePart)-5:]
		targetZonePart := timePart[len(timePart)-5:]
		if !bytes.Equal(cacheZonePart, targetZonePart) {
			return p.regularTimeParse(timePart)
		}
	}

	{
		cacheDatePart := p.lastFullyParsedTimePart[:len(p.lastFullyParsedTimePart)-15]
		targetDatePart := timePart[:len(timePart)-15]
		if !bytes.Equal(cacheDatePart, targetDatePart) {
			return p.regularTimeParse(timePart)
		}
	}
	targetHourPart := timePart[len(timePart)-14 : len(timePart)-12]
	targetMinutePart := timePart[len(timePart)-11 : len(timePart)-9]
	targetSecondPart := timePart[len(timePart)-8 : len(timePart)-6]

	hour, hourErr := p.parseInt64(targetHourPart)
	if hourErr != nil {
		return 0, hourErr
	}
	minute, minuteErr := p.parseInt64(targetMinutePart)
	if minuteErr != nil {
		return 0, minuteErr
	}
	second, secondErr := p.parseInt64(targetSecondPart)
	if secondErr != nil {
		return 0, secondErr
	}

	secondOfDayUnixTime := (hour * 3600) + (minute * 60) + second
	return p.lastFullyParsedTimeStartOfTheDayUnixTime + secondOfDayUnixTime, nil
}

/*
This parser is really slow (due to generalized layout) [determined by profiling via pprof].
So we use it only to parse date and timezone and cache results.
We parse using time.UTC zone to avoid allocations of *time.Location
*/
func (p *LineToStoreRecordParser) regularTimeParse(timePart []byte) (int64, error) {
	timePartStr := *(*string)(unsafe.Pointer(&timePart)) // bytes to string without potential allocation
	t, parsingErr := time.ParseInLocation("02/Jan/2006:15:04:05 -0700", timePartStr, time.UTC)
	if parsingErr != nil {
		return 0, parsingErr
	}
	p.lastFullyParsedTimeStartOfTheDayUnixTime = time.Date(
		t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC,
	).Unix()
	p.lastFullyParsedTimePart = make([]byte, len(timePart))
	copy(p.lastFullyParsedTimePart, timePart)
	return t.Unix(), nil
}

/*
Simplified int parser that is faster than `strconv.ParseInt` etc.
We don't need to parse signs like `-` or `+` and we don't need any scientific notation.
And our base is always 10.
*/
func (p *LineToStoreRecordParser) parseInt64(intPart []byte) (int64, error) {
	number := int64(0)
	for _, d := range intPart {
		if d < '0' || d > '9' {
			return 0, fmt.Errorf("can't parse int: `%s`", string(intPart))
		}
		number *= 10
		number += int64(d - '0')
	}
	return number, nil
}

func (p *LineToStoreRecordParser) internSectionString(sectionPart []byte) string {
	if p.sectionInternCacheSize == 0 {
		return string(sectionPart)
	}
	if len(p.sectionPartsInternCache) > p.sectionInternCacheSize {
		p.sectionPartsInternCache = nil
	}

	sectionPartStr := *(*string)(unsafe.Pointer(&sectionPart)) // bytes to string without potential allocation
	p.stringInternHash.Reset()
	_, _ = p.stringInternHash.Write(sectionPart)
	sectionHash := p.stringInternHash.Sum32()
	partFromCache, ok := p.sectionPartsInternCache[sectionHash]
	if ok && sectionPartStr == partFromCache {
		return partFromCache
	}
	sectionString := string(sectionPart)
	p.sectionPartsInternCache[sectionHash] = sectionString
	return sectionString
}
