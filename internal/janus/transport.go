package janus

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// janusDestination is the official LoginService endpoint. The server name is
// pinned in the TLS config; routing through a proxy changes only the transport
// path, never the verified identity.
const janusDestination = "aws.api.snapchat.com:443"

// TransportOptions configures the Janus client connection.
type TransportOptions struct {
	// ProxyAddr is an optional SOCKS5 proxy address (host:port) through which
	// all connections are tunneled, e.g. "127.0.0.1:9150" for Tor. When empty,
	// the client connects directly.
	ProxyAddr string
	// SurfaceServerErrors returns the raw gRPC status (including the server's
	// message text) instead of the sanitized error. Enable only for explicit
	// debugging against your own account; server messages may contain content
	// that should not be logged.
	SurfaceServerErrors bool
	// RPCTimeout bounds a single LoginWithPassword call. Zero means 10s (the
	// historical default). Cold Tor circuits can take longer than 10s to
	// become ready, so callers behind a fresh SOCKS5 proxy may raise this.
	RPCTimeout time.Duration
}

// Transport is a minimal gRPC client for the Janus LoginService.
type Transport struct {
	connection          *grpc.ClientConn
	surfaceServerErrors bool
	rpcTimeout          time.Duration
}

// protoMessage adapts raw bytes to the grpc proto codec.
type protoMessage []byte

type protoCodec struct{}

func (protoCodec) Name() string { return "proto" }

func (protoCodec) Marshal(value any) ([]byte, error) {
	message, ok := value.(*protoMessage)
	if !ok || message == nil {
		return nil, errors.New("invalid Janus protobuf message type")
	}
	return []byte(*message), nil
}

func (protoCodec) Unmarshal(data []byte, value any) error {
	message, ok := value.(*protoMessage)
	if !ok || message == nil {
		return errors.New("invalid Janus protobuf message type")
	}
	*message = append((*message)[:0], data...)
	return nil
}

// NewTransport creates a TLS-verified client for the official destination,
// requiring the recovered Android Snapchat user-agent format. It connects
// directly; use NewTransportWithOptions to route through a SOCKS5 proxy.
func NewTransport(userAgent string) (*Transport, error) {
	return NewTransportWithOptions(userAgent, TransportOptions{})
}

// NewTransportWithOptions creates a TLS-verified client, optionally tunneling
// every connection through the configured SOCKS5 proxy. The destination host
// name is still sent to the proxy for resolution and the TLS server identity
// remains pinned to aws.api.snapchat.com.
func NewTransportWithOptions(userAgent string, options TransportOptions) (*Transport, error) {
	if !strings.HasPrefix(userAgent, "Snapchat/") || !strings.Contains(userAgent, "; Android ") || !utf8.ValidString(userAgent) || strings.ContainsAny(userAgent, "\r\n\x00") {
		return nil, errors.New("Janus requires an Android Snapchat client user agent")
	}
	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			ServerName: "aws.api.snapchat.com",
			MinVersion: tls.VersionTLS12,
		})),
		grpc.WithUserAgent(userAgent),
		grpc.WithDisableRetry(),
		grpc.WithDisableServiceConfig(),
	}
	if options.ProxyAddr != "" {
		dialOptions = append(dialOptions, grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			// addr is the passthrough target (aws.api.snapchat.com:443). The
			// SOCKS5 CONNECT sends the host name to the proxy so resolution
			// happens there, while the TLS identity stays pinned to the name.
			return socks5DialContext(ctx, options.ProxyAddr, addr)
		}))
	}
	// passthrough:/// prevents gRPC's default DNS resolver from resolving the
	// destination locally. The domain name reaches the dialer verbatim, so a
	// SOCKS5 proxy performs resolution (and no DNS leaks outside the proxy).
	connection, err := grpc.NewClient("passthrough:///"+janusDestination, dialOptions...)
	if err != nil {
		return nil, errors.New("could not initialize Janus gRPC client")
	}
	return &Transport{connection: connection, surfaceServerErrors: options.SurfaceServerErrors, rpcTimeout: options.RPCTimeout}, nil
}

// Close releases the underlying connection.
func (transport *Transport) Close() error {
	return transport.connection.Close()
}

// LoginWithPassword performs the password-login RPC and returns the raw
// response bytes. RPC errors are sanitized so server message contents are
// never propagated into callers that might log them.
func (transport *Transport) LoginWithPassword(ctx context.Context, request []byte) ([]byte, error) {
	if len(request) == 0 {
		return nil, errors.New("Janus password request is empty")
	}
	rpcTimeout := transport.rpcTimeout
	if rpcTimeout <= 0 {
		rpcTimeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(
		"accept-language", "en-US",
		"x-snap-janus-request-created-at", strconv.FormatInt(time.Now().UnixMilli(), 10),
	))
	payload := protoMessage(request)
	var response protoMessage
	err := transport.connection.Invoke(ctx, LoginWithPasswordPath, &payload, &response, grpc.ForceCodec(protoCodec{}))
	if err != nil {
		code := status.Code(err)
		if transport.surfaceServerErrors {
			return nil, err
		}
		// Transport-level failures describe the client's own connection
		// attempt (dial, proxy, TLS, timeout) and carry no server response
		// content, so they are surfaced for diagnosis. Application-level
		// statuses stay sanitized to avoid leaking server message contents.
		if code == codes.Unavailable || code == codes.DeadlineExceeded || code == codes.Canceled {
			return nil, fmt.Errorf("Janus password-login RPC %s: %w", code, err)
		}
		return nil, status.Error(code, "Janus password-login RPC failed")
	}
	return response, nil
}
