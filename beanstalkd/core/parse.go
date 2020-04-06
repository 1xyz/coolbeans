package core

import (
	"bytes"
	"io"
)

func Discard(rdr io.Reader, b []byte) ([]byte, error) {
	mr := io.MultiReader(bytes.NewReader(b), rdr)
	isLastByteCarriageReturn := false

	for {
		b := make([]byte, 1024)
		n, err := mr.Read(b)
		if err != nil {
			return nil, err
		}

		if n < 0 {
			panic("un-expected: negative value for n")
		} else if n == 0 {
			return nil, ErrNoData
		} else {
			// check the case when \r\n encounters on a buffer boundary
			// check to see if the first byte is a newline
			if isLastByteCarriageReturn && b[0] == '\n' {
				// discard the first byte (\n) and return the rest
				return b[1:n], nil
			}

			if _, right, ok := split(b[0:n]); ok {
				return right, nil
			}

			isLastByteCarriageReturn = b[n-1] == '\r'
		}
	}
}

func Scan(rdr io.Reader, b []byte, limitBytes int) ([]byte, []byte, error) {
	buf := make([]byte, 0)
	if len(b) > 0 {
		if left, right, ok := split(b); ok {
			return clone(left), clone(right), nil
		} else {
			buf = append(buf, b...)
		}
	}

	// set to true if the last byte in last read call scanned is \r
	isLastByteCarriageReturn := len(buf) > 0 && buf[len(buf)-1] == '\r'
	// read buffer limitBytes in bytes
	bufSize := readBufferSizeBytes
	if bufSize > limitBytes {
		bufSize = limitBytes
	}

	for len(buf) < limitBytes {
		b := make([]byte, bufSize)
		n, err := rdr.Read(b)
		if err != nil {
			return nil, nil, err
		}

		if n < 0 {
			panic("un-expected: negative value for n")
		} else if n == 0 {
			return nil, buf, ErrNoData
		} else {
			// check the case when \r\n encounters on a buffer boundary
			// check to see if the first byte is a newline
			if isLastByteCarriageReturn && b[0] == '\n' {
				// discard the first byte (\n) and return the rest
				return buf[:len(buf)-1], b[1:n], nil
			}

			if left, right, ok := split(b[0:n]); ok {
				buf = append(buf, left...)
				return buf, right, nil
			}

			buf = append(buf, b[0:n]...)
			isLastByteCarriageReturn = b[n-1] == '\r'
		}
	}

	return nil, nil, ErrDelimiterMissing
}

// ScanJobData - Scans the provided "b" byte slice in search of a newline
// (\\r\\n) delimiter. If the provided slice does'nt have a newline, then
// scan the reader upto "MaxJobDataSizeBytes-len(b)" bytes in search of a
// delimiter
// Returns a triple of command, extra byte slice and an error
// The extra byte slice are any additional bytes read after encountering the
// delimiter.
func ScanJobData(rdr io.Reader, b []byte) ([]byte, []byte, error) {
	// Note this temporarily create a byte-array upto 16Kb in size
	return Scan(rdr, b, MaxJobDataSizeBytes)
}

// ScanCmdLine - Scans the provided "b" byte slice in search of a newline
// (\\r\\n) delimiter. If the provided slice does not have a newline,
// then scan the reader upto "MaxCmdSizeBytes-len(b)" bytes in search of
// the delimiter.
// Returns a triple of command, extra byte slice and an error
// The extra byte slice are any additional bytes read after encountering the
// delimiter.
func ScanCmdLine(rdr io.Reader, b []byte) ([]byte, []byte, error) {
	return Scan(rdr, b, MaxCmdSizeBytes)
}

// scans the input byte slice in search of a newline (\\r\\n) delimiter
// returns a triple of left and right hand slices and bool indicating if
// a result was found
func split(b []byte) ([]byte, []byte, bool) {
	if loc := DelimRe.FindIndex(b); loc != nil {
		return b[0:loc[0]], b[loc[1]:], true
	} else {
		return nil, nil, false
	}
}

// return a clone of  the src byte slice
func clone(src []byte) []byte {
	// https://github.com/go101/go101/wiki/How-to-efficiently-clone-a-slice%3F
	return append(src[:0:0], src...)
}
