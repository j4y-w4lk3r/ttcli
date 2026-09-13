package ticktick

import (
	"errors"
	"fmt"
	"io"
	"net"
	"syscall"
	"testing"
)

func TestIsRetryableNetErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"connection reset", fmt.Errorf(`read tcp 192.168.1.44:42526->13.56.51.85:443: read: connection reset by peer`), true},
		{"timeout", &net.DNSError{IsTimeout: true}, true},
		{"eof", io.EOF, true},
		{"auth error", errors.New("auth rejected (HTTP 401)"), false},
		{"syscall reset", &net.OpError{Err: syscall.ECONNRESET}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRetryableNetErr(tc.err); got != tc.want {
				t.Fatalf("isRetryableNetErr(%v)=%v want %v", tc.err, got, tc.want)
			}
		})
	}
}
