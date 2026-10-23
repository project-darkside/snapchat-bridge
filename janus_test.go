package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestJanusTransportLiveProbe(t *testing.T) {
	if os.Getenv("SNAPCHAT_JANUS_TRANSPORT_PROBE") != "1" {
		t.Skip("set SNAPCHAT_JANUS_TRANSPORT_PROBE=1 for the credential-free official RPC probe")
	}
	transport, err := newJanusTransport("Snapchat/14.29.0.50 (ProtocolProbe; Android 17#0#37; gzip)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = transport.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var request, response janusProtoMessage
	err = transport.connection.Invoke(ctx, janusPasswordLoginPath, &request, &response, grpc.ForceCodec(janusProtoCodec{}))
	if status.Code(err) != codes.InvalidArgument || status.Convert(err).Message() != "invalidRequest" {
		t.Fatalf("empty official RPC did not reach request validation (gRPC code %s)", status.Code(err))
	}
}

func TestJanusTransportRejectsInvalidUserAgent(t *testing.T) {
	for _, userAgent := range []string{"", "protocol-probe/1", "Snapchat/", "Snapchat/1 (Phone; Android 17)\r\nInjected: header", "Snapchat/1 (Phone; Android \xff)", "Snapchat/1 (Phone; Android 17)\x00"} {
		transport, err := newJanusTransport(userAgent)
		if err == nil || transport != nil {
			t.Fatal("invalid client profile must not create a transport")
		}
	}
}

func TestJanusTransport(t *testing.T) {
	const userAgent = "Snapchat/14.29.0.50 (ProtocolProbe; Android 17#0#37; gzip)"
	tests := []struct {
		name       string
		response   []byte
		rpcError   error
		wantStatus uint64
		wantErr    bool
	}{
		{"success envelope", []byte{8, 1, 18, 2, 10, 0}, nil, 1, false},
		{"challenge", []byte{8, 2, 26, 0}, nil, 2, false},
		{"malformed response", []byte{8, 1}, nil, 0, true},
		{"RPC rejection", nil, status.Error(codes.Unauthenticated, "server-secret-marker"), 0, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			listener := bufconn.Listen(1 << 20)
			t.Cleanup(func() { _ = listener.Close() })
			observed := make(chan struct {
				method  string
				request []byte
				headers metadata.MD
			}, 1)
			server := grpc.NewServer(grpc.ForceServerCodec(janusProtoCodec{}), grpc.UnknownServiceHandler(func(_ any, stream grpc.ServerStream) error {
				method, _ := grpc.MethodFromServerStream(stream)
				var request janusProtoMessage
				if err := stream.RecvMsg(&request); err != nil {
					return err
				}
				headers, _ := metadata.FromIncomingContext(stream.Context())
				observed <- struct {
					method  string
					request []byte
					headers metadata.MD
				}{method, request, headers}
				if test.rpcError != nil {
					return test.rpcError
				}
				response := janusProtoMessage(test.response)
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
			transport := &janusTransport{connection: connection}
			t.Cleanup(func() { _ = transport.Close() })
			request, err := marshalJanusPasswordRequest(janusUsername, "u", "synthetic-password", nil)
			if err != nil {
				t.Fatal(err)
			}
			envelope, err := transport.loginWithPassword(context.Background(), request)
			if (err != nil) != test.wantErr || envelope.Status != test.wantStatus {
				t.Fatalf("response status/error = %d/%v, want %d/error=%v", envelope.Status, err, test.wantStatus, test.wantErr)
			}
			if err != nil && strings.Contains(err.Error(), "server-secret-marker") {
				t.Fatal("RPC error exposed server response contents")
			}
			if test.rpcError != nil && status.Code(err) != status.Code(test.rpcError) {
				t.Fatal("RPC status was lost")
			}
			select {
			case call := <-observed:
				if call.method != janusPasswordLoginPath || !bytes.Equal(call.request, request) {
					t.Fatal("RPC method or serialized credentials changed in transit")
				}
				if !strings.HasPrefix(strings.Join(call.headers.Get("user-agent"), ""), userAgent) {
					t.Fatal("missing application user-agent prefix")
				}
				created, err := strconv.ParseInt(strings.Join(call.headers.Get("x-snap-janus-request-created-at"), ""), 10, 64)
				if err != nil || time.Since(time.UnixMilli(created)).Abs() > time.Minute {
					t.Fatal("missing or stale request timestamp")
				}
				if strings.Join(call.headers.Get("accept-language"), "") != "en-US" {
					t.Fatal("missing language metadata")
				}
			default:
				t.Fatal("gRPC handler did not observe the request")
			}
		})
	}
}

func TestJanusLoginEnvelope(t *testing.T) {
	tests := []struct {
		name    string
		wire    string
		status  uint64
		payload int
		wantErr bool
	}{
		{"success envelope", "080112020a00", 1, 2, false},
		{"challenge is not success", "08021a00", 2, 3, false},
		{"unknown status", "0863", 99, 0, false},
		{"unknown field", "08021a00720100", 2, 3, false},
		{"last oneof wins", "080212020a001a00", 2, 3, false},
		{"last status wins", "080108021a00", 2, 3, false},
		{"empty", "", 0, 0, true},
		{"missing status", "12020a00", 0, 0, true},
		{"missing success payload", "0801", 0, 0, true},
		{"empty success payload", "08011200", 0, 0, true},
		{"conflicting success oneof", "080112020a001a00", 0, 0, true},
		{"wrong status type", "0a0101", 0, 0, true},
		{"wrong payload type", "08011001", 0, 0, true},
		{"truncated status", "0880", 0, 0, true},
		{"truncated payload", "080112020a", 0, 0, true},
		{"invalid unknown field", "08020f", 0, 0, true},
		{"invalid tag", "00", 0, 0, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wire, err := hex.DecodeString(test.wire)
			if err != nil {
				t.Fatal(err)
			}
			envelope, err := parseJanusLoginEnvelope(wire)
			if (err != nil) != test.wantErr {
				t.Fatalf("parse error = %v, want error %v", err, test.wantErr)
			}
			if envelope.Status != test.status || int(envelope.PayloadTag) != test.payload {
				t.Fatalf("status/tag = %d/%d, want %d/%d", envelope.Status, envelope.PayloadTag, test.status, test.payload)
			}
		})
	}
}

func TestJanusPasswordRequestMatchesAPK(t *testing.T) {
	tests := []struct {
		name       string
		kind       janusIdentifierType
		identifier string
		want       string
	}{
		{"username", janusUsername, "bridge-test-user", "0a106272696467652d746573742d7573657222136e6f742d612d7265616c2d70617373776f7264"},
		{"email", janusEmail, "bridge-test@example.invalid", "121b6272696467652d74657374406578616d706c652e696e76616c696422136e6f742d612d7265616c2d70617373776f7264"},
		{"phone", janusPhone, "+15555550100", "1a0c2b313535353535353031303022136e6f742d612d7265616c2d70617373776f7264"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, err := marshalJanusPasswordRequest(test.kind, test.identifier, "not-a-real-password", nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := hex.EncodeToString(request); got != test.want {
				t.Errorf("request = %s, want APK bytes %s", got, test.want)
			}
		})
	}
}

func TestJanusPasswordRequestPrerequisites(t *testing.T) {
	request, err := marshalJanusPasswordRequest(janusUsername, "u", " p ", []byte{0x5a, 0x02, 0x01, 0x02})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := hex.EncodeToString(request), "0a017522032070207a045a020102"; got != want {
		t.Errorf("request = %s, want %s", got, want)
	}
}

func TestJanusPasswordRequestRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name          string
		kind          janusIdentifierType
		identifier    string
		password      string
		prerequisites []byte
	}{
		{"identifier type zero", 0, "user", "secret-marker", nil},
		{"identifier type four", 4, "user", "secret-marker", nil},
		{"empty identifier", janusUsername, "", "secret-marker", nil},
		{"empty password", janusUsername, "user", "", nil},
		{"invalid identifier UTF-8", janusUsername, "\xff", "secret-marker", nil},
		{"invalid password UTF-8", janusUsername, "user", "secret-marker\xff", nil},
		{"truncated prerequisite", janusUsername, "user", "secret-marker", []byte{0x5a, 2, 1}},
		{"invalid prerequisite tag", janusUsername, "user", "secret-marker", []byte{0}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, err := marshalJanusPasswordRequest(test.kind, test.identifier, test.password, test.prerequisites)
			if err == nil || request != nil {
				t.Fatal("expected a validation error and no serialized credentials")
			}
			if strings.Contains(err.Error(), "secret-marker") {
				t.Fatal("validation error exposed the password")
			}
		})
	}
}
