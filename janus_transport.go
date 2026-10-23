package main

import (
	"context"
	"crypto/tls"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type janusTransport struct {
	connection *grpc.ClientConn
}

type janusProtoMessage []byte

type janusProtoCodec struct{}

func (janusProtoCodec) Name() string {
	return "proto"
}

func (janusProtoCodec) Marshal(value any) ([]byte, error) {
	message, ok := value.(*janusProtoMessage)
	if !ok || message == nil {
		return nil, errors.New("invalid Janus protobuf message type")
	}
	return []byte(*message), nil
}

func (janusProtoCodec) Unmarshal(data []byte, value any) error {
	message, ok := value.(*janusProtoMessage)
	if !ok || message == nil {
		return errors.New("invalid Janus protobuf message type")
	}
	*message = append((*message)[:0], data...)
	return nil
}

func newJanusTransport(userAgent string) (*janusTransport, error) {
	if !strings.HasPrefix(userAgent, "Snapchat/") || !strings.Contains(userAgent, "; Android ") || !utf8.ValidString(userAgent) || strings.ContainsAny(userAgent, "\r\n\x00") {
		return nil, errors.New("Janus requires an Android Snapchat client user agent")
	}
	connection, err := grpc.NewClient("aws.api.snapchat.com:443",
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			ServerName: "aws.api.snapchat.com",
			MinVersion: tls.VersionTLS12,
		})),
		grpc.WithUserAgent(userAgent),
		grpc.WithDisableRetry(),
		grpc.WithDisableServiceConfig(),
	)
	if err != nil {
		return nil, errors.New("could not initialize Janus gRPC client")
	}
	return &janusTransport{connection: connection}, nil
}

func (transport *janusTransport) Close() error {
	return transport.connection.Close()
}

func (transport *janusTransport) loginWithPassword(ctx context.Context, request []byte) (janusLoginEnvelope, error) {
	if len(request) == 0 {
		return janusLoginEnvelope{}, errors.New("Janus password request is empty")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(
		"accept-language", "en-US",
		"x-snap-janus-request-created-at", strconv.FormatInt(time.Now().UnixMilli(), 10),
	))
	payload := janusProtoMessage(request)
	var response janusProtoMessage
	err := transport.connection.Invoke(ctx, janusPasswordLoginPath, &payload, &response, grpc.ForceCodec(janusProtoCodec{}))
	if err != nil {
		return janusLoginEnvelope{}, status.Error(status.Code(err), "Janus password-login RPC failed")
	}
	return parseJanusLoginEnvelope(response)
}
