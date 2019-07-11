package file

import (
	"encoding/base64"
	"github.com/storozhukBM/logstat/common/log"
	"github.com/storozhukBM/logstat/common/test"
	"io"
	"io/ioutil"
	"math/rand"
	"testing"
)

func TestFileReader(t *testing.T) {
	t.Parallel()
	const lineBufSize = 16 * 1024

	tmpFile, tmpFileErr := ioutil.TempFile("", "test_file_reader")
	test.FailOnError(t, tmpFileErr)
	reader, readerErr := NewReader(tmpFile.Name(), lineBufSize)
	test.FailOnError(t, readerErr)
	defer log.OnError(reader.Close, "can't close file reader")

	{
		line, lineErr := reader.ReadOneLineAsSlice()
		test.Equals(t, []byte(nil), line, "line should be empty")
		test.Equals(t, io.EOF, lineErr, "file should be empty now")
	}

	appendToFile(t, tmpFile, []byte("first line"))
	appendToFile(t, tmpFile, []byte("second line"))

	{
		line, lineErr := reader.ReadOneLineAsSlice()
		test.Equals(t, []byte("first line"), line, "can't read line")
		test.FailOnError(t, lineErr)
	}
	{
		line, lineErr := reader.ReadOneLineAsSlice()
		test.Equals(t, []byte("second line"), line, "can't read line")
		test.FailOnError(t, lineErr)
	}
	{
		line, lineErr := reader.ReadOneLineAsSlice()
		test.Equals(t, []byte(nil), line, "line should be empty")
		test.Equals(t, io.EOF, lineErr, "file should be empty now")
	}

	test.FailOnError(t, tmpFile.Truncate(0))
	_, seekErr := tmpFile.Seek(0, io.SeekStart)
	test.FailOnError(t, seekErr)

	{
		line, lineErr := reader.ReadOneLineAsSlice()
		test.Equals(t, []byte(nil), line, "line should be empty")
		test.Equals(t, io.EOF, lineErr, "file should be empty now")
	}

	appendToFile(t, tmpFile, []byte("third line"))
	{
		line, lineErr := reader.ReadOneLineAsSlice()
		test.FailOnError(t, lineErr)
		test.Equals(t, []byte("third line"), line, "can't read line")
	}
	{
		line, lineErr := reader.ReadOneLineAsSlice()
		test.Equals(t, []byte(nil), line, "line should be empty")
		test.Equals(t, io.EOF, lineErr, "file should be empty now")
	}

	bigLine := make([]byte, 2*lineBufSize+12)
	rand.Read(bigLine)
	stringLikeLine := base64.StdEncoding.EncodeToString(bigLine)
	appendToFile(t, tmpFile, []byte(stringLikeLine))
	{
		line, lineErr := reader.ReadOneLineAsSlice()
		test.Equals(t, []byte(stringLikeLine), line, "can't read line")
		test.FailOnError(t, lineErr)
	}
	{
		line, lineErr := reader.ReadOneLineAsSlice()
		test.Equals(t, []byte(nil), line, "line should be empty")
		test.Equals(t, io.EOF, lineErr, "file should be empty now")
	}
}

func appendToFile(t testing.TB, writer io.Writer, line []byte) {
	_, err := writer.Write(line)
	test.FailOnError(t, err)
	_, err = writer.Write([]byte("\n"))
	test.FailOnError(t, err)
}
