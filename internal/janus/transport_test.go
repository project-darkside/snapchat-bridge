package janus

import (
	"context"
	"net"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

func TestTransportLoginWithPassword(t *testing.T) {
	const userAgent = "Snapchat/14.29.0.50 (JanusProbe; Android 15#0#35; gzip)"
	listener := bufconn.Listen(1 << 20)
	t.Cleanup(func() { _ = listener.Close() })

	observed := make(chan struct {
		method  string
		request []byte
		headers metadata.MD
	}, 1)
	server := grpc.NewServer(grpc.ForceServerCodec(protoCodec{}), grpc.UnknownServiceHandler(func(_ any, stream grpc.ServerStream) error {
		method, _ := grpc.MethodFromServerStream(stream)
		var request protoMessage
		if err := stream.RecvMsg(&request); err != nil {
			return err
		}
		headers, _ := metadata.FromIncomingContext(stream.Context())
		observed <- struct {
			method  string
			request []byte
			headers metadata.MD
		}{method, request, headers}
		response := protoMessage([]byte{8, 1, 18, 2, 10, 0})
		return stream.SendMsg(&response)
	}))
	t.Cleanup(server.Stop)
	go func() { _ = server.Serve(listener) }()

	connection, err := grpc.NewClient("passthrough:///janus-test",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		grpc.WithUserAgent(userAgent),
	)
	if err != nil {
		t.Fatal(err)
	}
	transport := &Transport{connection: connection}
	t.Cleanup(func() { _ = transport.Close() })

	request := &PasswordRequest{Identifier: "u", IdentifierKind: Username, Password: "p"}
	encoded, err := request.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	response, err := transport.LoginWithPassword(context.Background(), encoded)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := ParseLoginEnvelope(response)
	if err != nil {
		t.Fatal(err)
	}
	if envelope.Status != 1 || envelope.PayloadTag != 2 {
		t.Fatalf("envelope = %+v", envelope)
	}

	select {
	case call := <-observed:
		if call.method != LoginWithPasswordPath || string(call.request) != string(encoded) {
			t.Fatal("RPC method or serialized request changed in transit")
		}
		if !strings.HasPrefix(strings.Join(call.headers.Get("user-agent"), ""), userAgent) {
			t.Fatal("missing application user-agent prefix")
		}
		if strings.Join(call.headers.Get("accept-language"), "") != "en-US" {
			t.Fatal("missing language metadata")
		}
		if strings.Join(call.headers.Get("x-snap-janus-request-created-at"), "") == "" {
			t.Fatal("missing request-created-at metadata")
		}
	default:
		t.Fatal("gRPC handler did not observe the request")
	}
}

func TestTransportRejectsEmptyRequest(t *testing.T) {
	transport, err := NewTransport("Snapchat/14.29.0.50 (JanusProbe; Android 15#0#35; gzip)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = transport.Close() })
	if _, err := transport.LoginWithPassword(context.Background(), nil); err == nil {
		t.Fatal("empty request must be rejected")
	}
}

func TestNewTransportRejectsInvalidUserAgent(t *testing.T) {
	for _, userAgent := range []string{"", "probe/1", "Snapchat/", "Snapchat/1 (Phone; Android 17)\r\nInjected: x"} {
		transport, err := NewTransport(userAgent)
		if err == nil || transport != nil {
			t.Fatal("invalid client profile must not create a transport")
		}
	}
}
