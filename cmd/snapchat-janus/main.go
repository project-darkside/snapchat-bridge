// Command snapchat-janus is a diagnostic CLI for the Snapchat Janus
// password-login RPC recovered from the installed Android 14.29.0.50 build.
//
// It assembles the recovered n070 request (identifier oneof, password,
// optional attempt/source flags, Fidelius init, COF metadata, and
// prerequisites), optionally sends it to the official endpoint, and decodes the
// recovered o070 response envelope and success payload.
//
// It is a reverse-engineering prototype, not a supported login client: device
// attestation, Fidelius key generation, and server acceptance remain
// unverified. Never supply real account credentials to this tool.
package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/beeper/snapchat-bridge/internal/janus"
)

const defaultUserAgent = "Snapchat/14.29.0.50 (JanusProbe; Android 15#0#35; gzip)"

// spec mirrors the request fields the CLI can assemble from a JSON file.
type spec struct {
	IdentifierKind string `json:"identifier_kind"`
	Identifier     string `json:"identifier"`
	Password       string `json:"password"`

	NumAttempts    *int64 `json:"num_attempts,omitempty"`
	PasswordSource *int32 `json:"password_source,omitempty"`
	WhatsApp       *bool  `json:"whatsapp,omitempty"`

	// Pre-encoded nested messages (hex or base64).
	FideliusInit  string `json:"fidelius_init,omitempty"`
	COFMetadata   string `json:"cof_metadata,omitempty"`
	Prerequisites string `json:"prerequisites,omitempty"`

	// Native attestation output, placed into prerequisite field 11.
	Attestation string `json:"attestation,omitempty"`

	// Structured Fidelius key material, assembled exactly as vo3.apply case 28.
	Fidelius *fideliusSpec `json:"fidelius,omitempty"`
}

type fideliusSpec struct {
	N1uH  []string `json:"n1u_h,omitempty"`  // base64/hex; v0u field 1
	N1uE  string   `json:"n1u_e,omitempty"`  // v0u field 3
	R241F string   `json:"r241_f,omitempty"` // k5u field 1 source, [26:91)
	R241C string   `json:"r241_c,omitempty"` // k5u field 2
	R241E string   `json:"r241_e,omitempty"` // k5u field 3
	R241I int64    `json:"r241_i,omitempty"` // k5u field 4
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("snapchat-janus", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	var (
		username = flags.String("username", "", "Snapchat username")
		email    = flags.String("email", "", "Snapchat email address")
		phone    = flags.String("phone", "", "Snapchat phone number")
		password = flags.String("password", "", "password (prefer --password-file or SNAPCHAT_PASSWORD)")
		passFile = flags.String("password-file", "", "read password from file ('-' for stdin)")
		specFile = flags.String("spec", "", "JSON request spec file ('-' for stdin)")

		userAgent = flags.String("user-agent", defaultUserAgent, "HTTP/2 user agent")

		proxy = flags.String("proxy", "127.0.0.1:9150", "SOCKS5 proxy address for all network calls (empty disables)")

		numAttempts    = flags.Int64("num-attempts", 0, "n070 field 5 (0 omits the field)")
		passwordSource = flags.Int("password-source", 0, "n070 field 9 (0 omits the field)")
		whatsapp       = flags.Bool("whatsapp", false, "n070 field 11")
		whatsappSet    = flags.Bool("set-whatsapp", false, "explicitly include n070 field 11")

		fideliusInitFile  = flags.String("fidelius-init-file", "", "pre-encoded v0u bytes (hex/base64)")
		cofMetadataFile   = flags.String("cof-metadata-file", "", "pre-encoded c3e bytes (hex/base64)")
		prerequisitesFile = flags.String("prerequisites-file", "", "pre-encoded fs60 bytes (hex/base64)")
		attestationFile   = flags.String("attestation-file", "", "native attestation output, placed in fs60 field 11")

		dumpRequest = flags.Bool("dump-request", false, "print the request and exit without sending")
		requestOut  = flags.String("request-out", "", "write request bytes to file")
		rawResponse = flags.Bool("raw", false, "print the raw response hex")
		timeout     = flags.Duration("timeout", 10*time.Second, "RPC timeout")
	)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}

	request, err := buildRequest(username, email, phone, password, passFile, specFile,
		numAttempts, passwordSource, whatsapp, whatsappSet,
		fideliusInitFile, cofMetadataFile, prerequisitesFile, attestationFile)
	if err != nil {
		return err
	}

	encoded, err := request.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	if *requestOut != "" {
		if err := os.WriteFile(*requestOut, encoded, 0o600); err != nil {
			return fmt.Errorf("failed to write request: %w", err)
		}
		fmt.Printf("request (%d bytes) written to %s\n", len(encoded), *requestOut)
	}
	if *dumpRequest {
		printRequest(encoded)
		return nil
	}

	transport, err := janus.NewTransportWithOptions(*userAgent, janus.TransportOptions{ProxyAddr: *proxy})
	if err != nil {
		return err
	}
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	response, err := transport.LoginWithPassword(ctx, encoded)
	if err != nil {
		return err
	}
	if *rawResponse {
		fmt.Printf("raw response (%d bytes): %s\n", len(response), hex.EncodeToString(response))
	}

	envelope, err := janus.ParseLoginEnvelope(response)
	if err != nil {
		return err
	}
	fmt.Printf("application status: %d (%s)\n", envelope.Status, envelope.StatusName())
	fmt.Printf("payload: field %d, %d bytes\n", envelope.PayloadTag, len(envelope.Payload))
	if len(envelope.Extra) > 0 {
		fmt.Printf("side channel (field 14): %d bytes\n", len(envelope.Extra))
	}
	if envelope.Status != 1 {
		fmt.Printf("payload hex: %s\n", hex.EncodeToString(envelope.Payload))
		return nil
	}

	session, err := janus.DecodeSuccess(envelope.Payload)
	if err != nil {
		return err
	}
	if len(session.Identity) > 0 {
		fmt.Printf("identity: %s\n", strings.Join(session.Identity, " | "))
	}
	if session.Primary != "" {
		fmt.Printf("primary: %s\n", session.Primary)
	}
	if session.ContainerStr != "" {
		fmt.Printf("container string: %s\n", session.ContainerStr)
	}
	fmt.Printf("container int: %d\n", session.ContainerInt)
	for i, token := range session.ScopedTokens {
		fmt.Printf("scoped token %d: scope=%q token=%q expiry=%d\n", i, token.Scope, token.Token, token.Expiry)
	}
	for i, extra := range session.SuccessBytes {
		fmt.Printf("success bytes %d: %s\n", i, hex.EncodeToString(extra))
	}
	return nil
}

func buildRequest(username, email, phone, password, passFile, specFile *string,
	numAttempts *int64, passwordSource *int, whatsapp, whatsappSet *bool,
	fideliusInitFile, cofMetadataFile, prerequisitesFile, attestationFile *string) (*janus.PasswordRequest, error) {
	request := &janus.PasswordRequest{}

	var loaded *spec
	if *specFile != "" {
		parsed, err := loadSpec(*specFile)
		if err != nil {
			return nil, err
		}
		loaded = parsed
	}

	// Read the password file once. A username:password file contributes both
	// the identifier (left of the first ':') and the password (the rest), so a
	// bare --password-file can satisfy both without --username.
	fileUsername, filePassword := "", ""
	fileProvided := *passFile != ""
	if fileProvided {
		user, pass, err := readPasswordFile(*passFile)
		if err != nil {
			return nil, err
		}
		fileUsername, filePassword = user, pass
	}

	kind, identifier, err := resolveIdentifier(loaded, *username, *email, *phone, fileUsername)
	if err != nil {
		return nil, err
	}
	request.Identifier = identifier
	request.IdentifierKind = kind

	secret, err := resolvePassword(loaded, *password, fileProvided, filePassword)
	if err != nil {
		return nil, err
	}
	request.Password = secret

	if loaded != nil {
		if loaded.NumAttempts != nil {
			numAttempts = loaded.NumAttempts
		}
		if loaded.PasswordSource != nil {
			value := int(*loaded.PasswordSource)
			passwordSource = &value
		}
		if loaded.WhatsApp != nil {
			value := *loaded.WhatsApp
			whatsapp = &value
			set := true
			whatsappSet = &set
		}
	}
	if *numAttempts > 0 {
		request.NumAttempts = numAttempts
	}
	if *passwordSource > 0 {
		value := int32(*passwordSource)
		request.PasswordSource = &value
	}
	if *whatsappSet {
		request.WhatsApp = whatsapp
	}

	if loaded != nil {
		if loaded.FideliusInit != "" {
			bytes, err := decodeBytes(loaded.FideliusInit)
			if err != nil {
				return nil, fmt.Errorf("fidelius_init: %w", err)
			}
			request.FideliusInit = bytes
		} else if loaded.Fidelius != nil {
			bytes, err := buildFidelius(loaded.Fidelius)
			if err != nil {
				return nil, err
			}
			request.FideliusInit = bytes
		}
		if loaded.COFMetadata != "" {
			bytes, err := decodeBytes(loaded.COFMetadata)
			if err != nil {
				return nil, fmt.Errorf("cof_metadata: %w", err)
			}
			request.COFMetadata = bytes
		}
		if loaded.Prerequisites != "" {
			bytes, err := decodeBytes(loaded.Prerequisites)
			if err != nil {
				return nil, fmt.Errorf("prerequisites: %w", err)
			}
			request.Prerequisites = bytes
		}
		if loaded.Attestation != "" {
			bytes, err := decodeBytes(loaded.Attestation)
			if err != nil {
				return nil, fmt.Errorf("attestation: %w", err)
			}
			prereq, err := (&janus.Prerequisites{Attestation: bytes}).Marshal()
			if err != nil {
				return nil, err
			}
			request.Prerequisites = prereq
		}
	}

	if *fideliusInitFile != "" {
		bytes, err := readBytes(*fideliusInitFile)
		if err != nil {
			return nil, fmt.Errorf("fidelius-init-file: %w", err)
		}
		request.FideliusInit = bytes
	}
	if *cofMetadataFile != "" {
		bytes, err := readBytes(*cofMetadataFile)
		if err != nil {
			return nil, fmt.Errorf("cof-metadata-file: %w", err)
		}
		request.COFMetadata = bytes
	}
	if *prerequisitesFile != "" {
		bytes, err := readBytes(*prerequisitesFile)
		if err != nil {
			return nil, fmt.Errorf("prerequisites-file: %w", err)
		}
		request.Prerequisites = bytes
	}
	if *attestationFile != "" {
		bytes, err := readBytes(*attestationFile)
		if err != nil {
			return nil, fmt.Errorf("attestation-file: %w", err)
		}
		prereq, err := (&janus.Prerequisites{Attestation: bytes}).Marshal()
		if err != nil {
			return nil, err
		}
		request.Prerequisites = prereq
	}

	return request, nil
}

func resolveIdentifier(loaded *spec, username, email, phone, fileUsername string) (janus.IdentifierKind, string, error) {
	values := map[janus.IdentifierKind]string{}
	if loaded != nil && loaded.Identifier != "" {
		kind, err := parseKind(loaded.IdentifierKind)
		if err != nil {
			return 0, "", err
		}
		values[kind] = loaded.Identifier
	}
	if username != "" {
		values[janus.Username] = username
	}
	if email != "" {
		values[janus.Email] = email
	}
	if phone != "" {
		values[janus.Phone] = phone
	}
	// Fall back to the username extracted from a username:password file only
	// when no explicit identifier was provided.
	if len(values) == 0 && fileUsername != "" {
		values[janus.Username] = fileUsername
	}
	if len(values) == 0 {
		return 0, "", errors.New("provide an identifier: --username, --email, --phone, a username:password --password-file, or a --spec file")
	}
	if len(values) > 1 {
		return 0, "", errors.New("provide exactly one identifier (username, email, or phone)")
	}
	for kind, value := range values {
		return kind, value, nil
	}
	return 0, "", errors.New("unreachable")
}

func parseKind(kind string) (janus.IdentifierKind, error) {
	switch strings.ToLower(kind) {
	case "username":
		return janus.Username, nil
	case "email":
		return janus.Email, nil
	case "phone":
		return janus.Phone, nil
	default:
		return 0, fmt.Errorf("identifier_kind must be username, email, or phone, got %q", kind)
	}
}

func resolvePassword(loaded *spec, password string, fileProvided bool, filePassword string) (string, error) {
	if password != "" {
		return password, nil
	}
	if fileProvided {
		return filePassword, nil
	}
	if loaded != nil && loaded.Password != "" {
		return loaded.Password, nil
	}
	if env := os.Getenv("SNAPCHAT_PASSWORD"); env != "" {
		return env, nil
	}
	return "", errors.New("provide a password via --password-file, SNAPCHAT_PASSWORD, --password, or --spec")
}

// readPasswordFile reads a credentials file. If it contains a ':' the part
// before the first ':' is the username and the remainder is the password;
// otherwise the whole content is the password and no username is returned.
func readPasswordFile(path string) (username, password string, err error) {
	content, err := readText(path)
	if err != nil {
		return "", "", err
	}
	if user, pass, ok := strings.Cut(content, ":"); ok {
		return user, pass, nil
	}
	return "", content, nil
}

func buildFidelius(spec *fideliusSpec) ([]byte, error) {
	n1uH := make([][]byte, 0, len(spec.N1uH))
	for _, entry := range spec.N1uH {
		bytes, err := decodeBytes(entry)
		if err != nil {
			return nil, fmt.Errorf("fidelius.n1u_h: %w", err)
		}
		n1uH = append(n1uH, bytes)
	}
	n1uE, err := decodeBytes(spec.N1uE)
	if err != nil {
		return nil, fmt.Errorf("fidelius.n1u_e: %w", err)
	}
	r241F, err := decodeBytes(spec.R241F)
	if err != nil {
		return nil, fmt.Errorf("fidelius.r241_f: %w", err)
	}
	r241C, err := decodeBytes(spec.R241C)
	if err != nil {
		return nil, fmt.Errorf("fidelius.r241_c: %w", err)
	}
	r241E, err := decodeBytes(spec.R241E)
	if err != nil {
		return nil, fmt.Errorf("fidelius.r241_e: %w", err)
	}
	message, err := janus.NewFideliusInit(n1uH, n1uE, r241F, r241C, r241E, spec.R241I)
	if err != nil {
		return nil, err
	}
	return message.Marshal()
}

func loadSpec(path string) (*spec, error) {
	data, err := readRaw(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read spec: %w", err)
	}
	var parsed spec
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse spec: %w", err)
	}
	return &parsed, nil
}

func readText(path string) (string, error) {
	data, err := readRaw(path)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(data), "\r\n"), nil
}

func readBytes(path string) ([]byte, error) {
	data, err := readRaw(path)
	if err != nil {
		return nil, err
	}
	return decodeBytes(strings.TrimSpace(string(data)))
}

func readRaw(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(bufio.NewReader(os.Stdin))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// decodeBytes accepts raw bytes, a hex string (optionally 0x-prefixed), or
// base64. Hex is preferred when the value parses as hex.
func decodeBytes(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return []byte{}, nil
	}
	hexValue := strings.TrimPrefix(value, "0x")
	hexValue = strings.TrimPrefix(hexValue, "0X")
	if len(hexValue)%2 == 0 && len(hexValue) > 0 && isHex(hexValue) {
		if decoded, err := hex.DecodeString(hexValue); err == nil {
			return decoded, nil
		}
	}
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
		return decoded, nil
	}
	return []byte(value), nil
}

func isHex(value string) bool {
	for _, char := range value {
		switch {
		case char >= '0' && char <= '9', char >= 'a' && char <= 'f', char >= 'A' && char <= 'F':
		default:
			return false
		}
	}
	return true
}

func printRequest(encoded []byte) {
	fmt.Printf("request (%d bytes):\n", len(encoded))
	fmt.Printf("  hex:    %s\n", hex.EncodeToString(encoded))
	fmt.Printf("  base64: %s\n", base64.StdEncoding.EncodeToString(encoded))
}
