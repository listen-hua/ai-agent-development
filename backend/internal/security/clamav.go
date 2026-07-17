package security

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"time"
)

type Scanner interface {
	Scan(context.Context, []byte) error
}
type NoopScanner struct{}

func (NoopScanner) Scan(context.Context, []byte) error { return nil }

type ClamAV struct{ Addr string }

func (c ClamAV) Scan(ctx context.Context, data []byte) error {
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", c.Addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	if _, err = conn.Write([]byte("zINSTREAM\x00")); err != nil {
		return err
	}
	reader := bytes.NewReader(data)
	buffer := make([]byte, 32*1024)
	for {
		n, readErr := reader.Read(buffer)
		if n > 0 {
			var size [4]byte
			binary.BigEndian.PutUint32(size[:], uint32(n))
			if _, err = conn.Write(size[:]); err != nil {
				return err
			}
			if _, err = conn.Write(buffer[:n]); err != nil {
				return err
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if _, err = conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return err
	}
	response, err := io.ReadAll(io.LimitReader(conn, 4096))
	if err != nil {
		return err
	}
	if strings.Contains(string(response), "FOUND") {
		return errors.New("malware detected by ClamAV")
	}
	if !strings.Contains(string(response), "OK") {
		return errors.New("unexpected ClamAV response")
	}
	return nil
}
