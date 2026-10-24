package janus

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"
)

// startEchoServer starts a TCP echo server and returns its address.
func startEchoServer(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}()
		}
	}()
	return listener.Addr().String()
}

// startSocks5Relay starts a minimal SOCKS5 server that accepts the greeting,
// parses the CONNECT request, records the requested target, relays the tunnel
// to that target, and echoes bytes back. It returns the proxy address and a
// channel carrying each recorded target.
func startSocks5Relay(t *testing.T, failConnect bool) (string, <-chan string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	targets := make(chan string, 8)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				if !readSOCKS5Greeting(conn) {
					return
				}
				host, port, ok := readSOCKS5Connect(conn)
				if !ok {
					return
				}
				targets <- net.JoinHostPort(host, portStr(port))
				if failConnect {
					writeSOCKS5Reply(conn, 0x05) // connection refused
					return
				}
				writeSOCKS5Reply(conn, 0x00) // success
				upstream, err := net.Dial("tcp", net.JoinHostPort(host, portStr(port)))
				if err != nil {
					return
				}
				defer upstream.Close()
				relay(conn, upstream)
			}()
		}
	}()
	return listener.Addr().String(), targets
}

func portStr(port int) string {
	return strconv.Itoa(port)
}

func readSOCKS5Greeting(conn net.Conn) bool {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil || header[0] != 0x05 {
		return false
	}
	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		return false
	}
	// Accept only no-auth (0x00); otherwise offer no acceptable method.
	method := byte(0xff)
	for _, candidate := range methods {
		if candidate == socks5NoAuthRequired {
			method = socks5NoAuthRequired
			break
		}
	}
	_, err := conn.Write([]byte{0x05, method})
	return err == nil && method == socks5NoAuthRequired
}

func readSOCKS5Connect(conn net.Conn) (string, int, bool) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil || header[0] != 0x05 || header[1] != 0x01 {
		return "", 0, false
	}
	var host string
	switch header[3] {
	case 0x01:
		ip := make([]byte, net.IPv4len)
		if _, err := io.ReadFull(conn, ip); err != nil {
			return "", 0, false
		}
		host = net.IP(ip).String()
	case 0x03:
		length := make([]byte, 1)
		if _, err := io.ReadFull(conn, length); err != nil {
			return "", 0, false
		}
		name := make([]byte, int(length[0]))
		if _, err := io.ReadFull(conn, name); err != nil {
			return "", 0, false
		}
		host = string(name)
	case 0x04:
		ip := make([]byte, net.IPv6len)
		if _, err := io.ReadFull(conn, ip); err != nil {
			return "", 0, false
		}
		host = net.IP(ip).String()
	default:
		return "", 0, false
	}
	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBytes); err != nil {
		return "", 0, false
	}
	return host, int(binary.BigEndian.Uint16(portBytes)), true
}

func writeSOCKS5Reply(conn net.Conn, reply byte) {
	// Reply: version 5, reply code, reserved, IPv4, 0.0.0.0:0.
	_, _ = conn.Write([]byte{0x05, reply, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
}

func relay(left, right net.Conn) {
	var once sync.Once
	closeBoth := func() {
		_ = left.Close()
		_ = right.Close()
	}
	go func() {
		_, _ = io.Copy(right, left)
		once.Do(closeBoth)
	}()
	_, _ = io.Copy(left, right)
	once.Do(closeBoth)
}

func TestSOCKS5DialContextRelaysAndSendsDomainTarget(t *testing.T) {
	echo := startEchoServer(t)
	echoHost, echoPort, err := net.SplitHostPort(echo)
	if err != nil {
		t.Fatal(err)
	}
	proxyAddr, targets := startSocks5Relay(t, false)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := socks5DialContext(ctx, proxyAddr, echo)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	payload := []byte("through-tor")
	if _, err := conn.Write(payload); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatal(err)
	}
	if string(reply) != string(payload) {
		t.Fatalf("echoed %q, want %q", reply, payload)
	}

	select {
	case target := <-targets:
		if target != net.JoinHostPort(echoHost, echoPort) {
			t.Fatalf("relay observed target %q, want %q", target, echo)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("relay never observed a CONNECT target")
	}
}

func TestSOCKS5DialContextRejectsRefusedConnect(t *testing.T) {
	proxyAddr, _ := startSocks5Relay(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := socks5DialContext(ctx, proxyAddr, "127.0.0.1:1"); err == nil {
		t.Fatal("refused CONNECT must return an error")
	}
}

func TestNewTransportWithOptionsRoutesThroughProxy(t *testing.T) {
	proxyAddr, targets := startSocks5Relay(t, false)

	transport, err := NewTransportWithOptions("Snapchat/14.29.0.50 (JanusProbe; Android 15#0#35; gzip)",
		TransportOptions{ProxyAddr: proxyAddr})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = transport.Close() })

	// The RPC is expected to fail (the relay does not terminate TLS for the
	// real destination); what matters is that the only CONNECT the proxy saw
	// was for the official destination, proving the call went through it.
	request := &PasswordRequest{Identifier: "u", IdentifierKind: Username, Password: "p"}
	encoded, err := request.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, _ = transport.LoginWithPassword(ctx, encoded)

	select {
	case target := <-targets:
		if target != janusDestination {
			t.Fatalf("proxy observed target %q, want %q", target, janusDestination)
		}
	default:
		t.Fatal("proxy never observed a CONNECT; traffic did not go through the SOCKS5 proxy")
	}
}
