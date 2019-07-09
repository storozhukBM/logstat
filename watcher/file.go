package watcher

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/storozhukBM/logstat/common/cmp"
	"github.com/storozhukBM/logstat/common/log"
	"io"
	"os"
)

const minBufSize = 4 * 1024

/*
Component used to read new lines from file

Responsibilities:
 - open file and Close target file
 - track current reading offset
 - detect that file was rotated and start from the beginning of the new file

Attention:
 - `ReadOneLineAsSlice` returns a view to internal reading buffer
to avoid copying and pressure on GC. This view is only valid before the next
`ReadOneLineAsSlice` call. If you need some parts of it to remain accessible,
copy required parts.
- call `Close` function to free managed resources
*/
type FileReader struct {
	fileName      string
	readerBufSize uint

	initialized          bool
	currentOffset        int64
	currentFile          *os.File
	currentReader        *bufio.Reader
	overflowForLongLines *bytes.Buffer
}

func NewFileReader(fileName string, readerBufSize uint) (*FileReader, error) {
	if fileName == "" {
		return nil, fmt.Errorf("fileName can't be empty")
	}
	result := &FileReader{
		fileName:             fileName,
		readerBufSize:        cmp.MaxUInt(readerBufSize, minBufSize),
		overflowForLongLines: bytes.NewBuffer(nil),
	}
	return result, nil
}

func (f *FileReader) Close() error {
	if f.currentFile != nil {
		return f.currentFile.Close()
	}
	return nil
}

func (f *FileReader) ReadOneLineAsSlice() ([]byte, error) {
	fileErr := f.prepareFileReadAndDetectRotation()
	if fileErr != nil {
		return nil, fileErr
	}
	f.overflowForLongLines.Reset()
	returnOverflow := false
	for {
		line, isPrefix, readErr := f.currentReader.ReadLine()
		if readErr != nil {
			return nil, readErr
		}
		f.currentOffset += int64(len(line))

		if isPrefix {
			log.Debug("using overflow buf: %v; bufSize: %v", f.fileName, f.overflowForLongLines.Cap())
			f.overflowForLongLines.Write(line)
			returnOverflow = true
			continue
		}
		if returnOverflow {
			f.overflowForLongLines.Write(line)
			return f.overflowForLongLines.Bytes(), nil
		}
		return line, nil
	}
}

func (f *FileReader) prepareFileReadAndDetectRotation() error {
	fileInitializationErr := f.openAndInitializeFile()
	if fileInitializationErr != nil {
		return fileInitializationErr
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

func (f *FileReader) openAndInitializeFile() error {
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

func (f *FileReader) checkTargetFileSize() (int64, error) {
	fileInfo, fileStatErr := f.currentFile.Stat()
	if fileStatErr != nil {
		return 0, fileStatErr
	}
	return fileInfo.Size(), nil
}
