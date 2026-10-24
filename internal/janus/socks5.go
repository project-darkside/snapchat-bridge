package janus

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

// socks5NoAuthRequired is the only authentication method offered: Tor's
// SOCKS5 listener (127.0.0.1:9150) accepts unauthenticated local connections.
const socks5NoAuthRequired = 0x00

// socks5DialContext opens a TCP connection to proxyAddr, performs the SOCKS5
// handshake, and requests a CONNECT to target (host:port). The target's DNS
// name is sent to the proxy so resolution happens through the proxy, not the
// local resolver. The returned connection is a raw tunnel ready for TLS.
func socks5DialContext(ctx context.Context, proxyAddr, target string) (net.Conn, error) {
	host, port, err := splitHostPort(target)
	if err != nil {
		return nil, fmt.Errorf("invalid SOCKS5 target %q: %w", target, err)
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid SOCKS5 target port %d", port)
	}

	dialer := &net.Dialer{}
	connection, err := dialer.DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("connect to SOCKS5 proxy %s: %w", proxyAddr, err)
	}

	type result struct {
		conn net.Conn
		err  error
	}
	done := make(chan result, 1)
	go func() {
		conn, err := socks5Handshake(connection, host, port)
		done <- result{conn: conn, err: err}
	}()

	select {
	case <-ctx.Done():
		_ = connection.Close()
		return nil, fmt.Errorf("SOCKS5 handshake with %s: %w", proxyAddr, ctx.Err())
	case res := <-done:
		if res.err != nil {
			_ = connection.Close()
			return nil, res.err
		}
		return res.conn, nil
	}
}

// socks5Handshake negotiates the greeting and CONNECT request and returns the
// established tunnel on success.
func socks5Handshake(connection net.Conn, host string, port int) (net.Conn, error) {
	// Bound the whole handshake so a dead proxy cannot hang the dialer forever.
	_ = connection.SetDeadline(time.Now().Add(10 * time.Second))

	// Greeting: version 5, one method, no authentication.
	if _, err := connection.Write([]byte{0x05, 0x01, socks5NoAuthRequired}); err != nil {
		return nil, fmt.Errorf("SOCKS5 greeting write: %w", err)
	}

	methodReply := make([]byte, 2)
	if _, err := io.ReadFull(connection, methodReply); err != nil {
		return nil, fmt.Errorf("SOCKS5 method reply: %w", err)
	}
	if methodReply[0] != 0x05 {
		return nil, fmt.Errorf("SOCKS5 proxy replied with version %d", methodReply[0])
	}
	if methodReply[1] != socks5NoAuthRequired {
		return nil, fmt.Errorf("SOCKS5 proxy requires unsupported authentication method 0x%02x", methodReply[1])
	}

	// CONNECT request: version 5, connect, reserved, domain (ATYP 0x03).
	request := []byte{0x05, 0x01, 0x00, 0x03, byte(len(host))}
	request = append(request, host...)
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(port))
	request = append(request, portBytes...)
	if _, err := connection.Write(request); err != nil {
		return nil, fmt.Errorf("SOCKS5 connect write: %w", err)
	}

	// Reply header: version, reply code, reserved, address type.
	reply := make([]byte, 4)
	if _, err := io.ReadFull(connection, reply); err != nil {
		return nil, fmt.Errorf("SOCKS5 connect reply: %w", err)
	}
	if reply[0] != 0x05 {
		return nil, fmt.Errorf("SOCKS5 connect reply version %d", reply[0])
	}
	if reply[1] != 0x00 {
		return nil, fmt.Errorf("SOCKS5 proxy refused CONNECT to %s: code 0x%02x", host, reply[1])
	}

	// Skip the bound address, whose length depends on the address type.
	var addressLength int
	switch reply[3] {
	case 0x01: // IPv4
		addressLength = net.IPv4len
	case 0x04: // IPv6
		addressLength = net.IPv6len
	case 0x03: // Domain: one length byte followed by the name.
		lengthByte := make([]byte, 1)
		if _, err := io.ReadFull(connection, lengthByte); err != nil {
			return nil, fmt.Errorf("SOCKS5 connect reply address: %w", err)
		}
		addressLength = int(lengthByte[0])
	default:
		return nil, fmt.Errorf("SOCKS5 connect reply has unknown address type 0x%02x", reply[3])
	}
	if _, err := io.CopyN(io.Discard, connection, int64(addressLength)+2); err != nil {
		return nil, fmt.Errorf("SOCKS5 connect reply address: %w", err)
	}

	_ = connection.SetDeadline(time.Time{})
	return connection, nil
}

// splitHostPort splits host:port, preserving IPv6 literal brackets.
func splitHostPort(target string) (string, int, error) {
	host, portString, err := net.SplitHostPort(target)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		return "", 0, err
	}
	return host, port, nil
}

var errSOCKS5Target = errors.New("invalid SOCKS5 target")
