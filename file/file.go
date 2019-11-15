package file

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/storozhukBM/logstat/common/cmp"
	"github.com/storozhukBM/logstat/common/log"
	"io"
	"os"
	"time"
)

const minBufSize = 4 * 1024

/*
A component used to read new lines from a file.
Under load in the hot path, this file reader should work with almost zero allocations.

Responsibilities:
	- open file and Close target file
	- track current reading offset
	- detect that file was rotated and start from the beginning of the new file

Attention:
	- `ReadOneLineAsSlice` returns a view to internal reading buffer to avoid copying and pressure on GC. This view is only valid before the next
	`ReadOneLineAsSlice` call. If you need some parts of it to remain accessible,
	copy required parts
	- call `Close` function to free managed resources
*/
type Reader struct {
	fileName      string
	readerBufSize uint

	input            chan []byte
	output           chan []byte
	currentBuf       []byte
	currentBufOffset int

	initialized          bool
	endReached           bool
	currentOffset        int64
	currentFile          *os.File
	currentReader        *bufio.Reader
	overflowForLongLines *bytes.Buffer
}

func NewReader(fileName string, readerBufSize uint) (*Reader, error) {
	if fileName == "" {
		return nil, fmt.Errorf("fileName can't be empty")
	}
	result := &Reader{
		fileName:             fileName,
		readerBufSize:        cmp.MaxUInt(readerBufSize, minBufSize),
		overflowForLongLines: bytes.NewBuffer(nil),

		input:  make(chan []byte, 5),
		output: make(chan []byte, 5),

		endReached: true,
	}
	for i := 0; i < 5; i++ {
		result.input <- make([]byte, result.readerBufSize)
	}
	go result.fetchAsync()
	return result, nil
}

func (f *Reader) Close() error {
	if f.currentFile != nil {
		return f.currentFile.Close()
	}
	return nil
}

func (f *Reader) ReadOneLineAsSlice() ([]byte, error) {
	f.overflowForLongLines.Reset()
	returnOverflow := false
	for {
		if f.currentBuf == nil {
			buf := <-f.output
			f.currentBuf = buf
		}

		line, readErr := f.readSlice('\n')
		if readErr != nil {
			f.overflowForLongLines.Write(line)
			returnOverflow = true
			f.currentBufOffset = 0
			f.currentBuf = f.currentBuf[0:cap(f.currentBuf)]
			f.input <- f.currentBuf
			f.currentBuf = nil
			continue
		}
		line = line[:len(line)-1]
		f.currentOffset += int64(len(line))
		if returnOverflow {
			f.overflowForLongLines.Write(line)
			return f.overflowForLongLines.Bytes(), nil
		}
		return line, nil
	}
}

func (f *Reader) readSlice(delim byte) (line []byte, err error) {
	i := bytes.IndexByte(f.currentBuf[f.currentBufOffset:], delim)
	end := f.currentBufOffset + i + 1
	if i < 0 {
		end = len(f.currentBuf)
		err = io.EOF
	}
	line = f.currentBuf[f.currentBufOffset:end]
	f.currentBufOffset = end
	return line, err
}

func (f *Reader) prepareFileReadAndDetectRotation() error {
	fileInitializationErr := f.openAndInitializeFile()
	if fileInitializationErr != nil {
		return fileInitializationErr
	}
	if !f.endReached {
		return nil
	}
	size, fileErr := f.checkTargetFileSize()
	if fileErr != nil {
		return fileErr
	}
	fileWasRotated := f.currentOffset > size
	if !fileWasRotated {
		return nil
	}
	log.Debug("file was rotated going to reopen: %v", f.fileName)

	prevFile := f.currentFile
	defer log.OnError(prevFile.Close, "can't Close file: %v", f.fileName)
	f.currentOffset = 0
	f.currentReader = nil
	f.currentFile = nil
	reopenFileErr := f.openAndInitializeFile()
	return reopenFileErr
}

func (f *Reader) openAndInitializeFile() error {
	if f.currentFile != nil {
		return nil
	}
	log.Debug("current file is nil: %v", f.fileName)

	file, fileOpenErr := os.Open(f.fileName)
	if fileOpenErr != nil {
		return fmt.Errorf("can't open file: %+v. error happened: %+v", f.fileName, fileOpenErr)
	}
	log.Debug("opened file: %v", f.fileName)
	f.currentFile = file
	if f.initialized {
		f.currentReader = bufio.NewReaderSize(file, int(f.readerBufSize))
		log.Debug("open reader directly without seek: %v", f.fileName)
		return nil
	}

	// This watcher is newly created, so we should seek to the end of the file initially
	size, fileErr := f.checkTargetFileSize()
	if fileErr != nil {
		return fileErr
	}
	log.Debug("file size: %v", size)
	f.currentOffset = size
	newOffset, seekErr := file.Seek(f.currentOffset, io.SeekStart)
	if seekErr != nil {
		return fmt.Errorf("can't seek to the current offset. file: %v; offset: %v", f.fileName, f.currentOffset)
	}
	log.Debug("seek new offset: %v", newOffset)
	if newOffset != f.currentOffset {
		return fmt.Errorf("offset missmatch. exp %v; act: %v", f.currentOffset, newOffset)
	}

	f.currentReader = bufio.NewReaderSize(file, int(f.readerBufSize))
	f.initialized = true
	return nil
}

func (f *Reader) checkTargetFileSize() (int64, error) {
	fileInfo, fileStatErr := f.currentFile.Stat()
	if fileStatErr != nil {
		return 0, fileStatErr
	}
	return fileInfo.Size(), nil
}

func (f *Reader) fetchAsync() {
	for {
		fileErr := f.prepareFileReadAndDetectRotation()
		if fileErr != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		buf := <-f.input
		buf = buf[:cap(buf)]
		n, readErr := f.currentReader.Read(buf)
		if readErr != nil {
			f.input <- buf
			time.Sleep(100 * time.Millisecond)
			continue
		}
		buf = buf[:n]
		f.output <- buf
	}
}
