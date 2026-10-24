package janus

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protowire"
)

// AttestationTypeLogin is the recovered LOGIN value for AfxRequest.Type
// (ydx field 3). The APK sets this before invoking the native attestation
// boundary.
const AttestationTypeLogin = 1

// AttestationPathLogin is the exact RPC path the 14.30.0.39 APK passes as the
// afx request's field 2 (ydx.c via ydx.d(String)) when the client-attestation
// interceptor classifies the request as LOGIN (nji0.c =
// "/snapchat.janus.api.LoginService/"). Recovered from n6o0.c -> n6o0.b.
const AttestationPathLogin = "/snapchat.janus.api.LoginService/LoginWithPassword"

// Login attesting endpoints. nji0 in the 14.30.0.39 APK classifies a request
// path as REGISTER when it equals "/loq/register_v2" or starts with
// "/snapchat.janus.api/RegistrationService/", and as LOGIN when it starts
// with "/snapchat.janus.api.LoginService/". kld.e maps LOGIN to afx type 1.
const (
	AttestationPathRegisterExact  = "/loq/register_v2"
	AttestationPathRegisterPrefix = "/snapchat.janus.api/RegistrationService/"
	AttestationPathLoginPrefix    = "/snapchat.janus.api.LoginService/"
)

// Recovered afx metadata entries (fg3 field 1 repeated kte) that dc81.q adds
// when Argos client attestation is enabled. The first three entries are
// hardcoded in the APK; the music entry reuses the ig3.e/ig3.f lists. When the
// ARGOS_SUPPORTED_ENDPOINTS_PREFIXES config is non-empty the APK appends a
// fourth kte whose field 1 is that string split on ",". When Argos is disabled
// dc81.q returns an empty fg3, so these entries are conditional on that flag.
var attestationMetadataEntries = []*AfxMetadataEntry{
	{
		B: []string{
			"/boosts-prod/",
			"/readreceipt-indexer/",
			"/streaming-collector/",
			"/suggest_friend_",
			"/snapchat.friending.server.FriendAction/",
			"/snapchat.telephony.api.PhoneEnrollmentService/",
		},
		C: []string{
			"/events_batch",
			"/snapchat.friending.server.ContactBook/FullSyncContactBookUpload",
		},
		T:    5,
		hasT: true,
		I:    true,
		hasI: true,
	},
	{
		B:    []string{"/messagingcoreservice.MessagingCoreService/"},
		T:    1,
		hasT: true,
		I:    false,
		hasI: true,
	},
	{
		B: []string{
			"/snapchat.music.music_service.MusicService/",
			"/music/snapchat.creativetools.compute.ComputeFeedService/",
			"/music/snapchat.creativetools.userdata.UserDataService/",
			"/snapchat.search.musicservice.SearchService/",
		},
		C: []string{
			"/GetMusicTrack",
			"/GetMusicTracks",
			"/GetPlaylist",
			"/GetPlaylists",
			"/GetFeaturedPlaylist",
			"/GetPickerLayout",
			"/GetPickerLayoutPage",
			"/GetMyCustomSoundsPlaylist",
			"/CheckIsAvailable",
			"/CreateCustomSound",
			"/UpdateCustomSound",
			"/DeleteCustomSound",
			"/UpdateOriginalSound",
		},
		T:    5,
		hasT: true,
		I:    true,
		hasI: true,
	},
}

// NewLoginAfxRequest builds the afx request the 14.30.0.39 APK mints for a
// LOGIN attestation: n6o0.c(path) calls n6o0.b(path, 1, null, new byte[0],
// null), which fills ydx field 2 (c) with the path, field 3 (t) with 1, field
// 4 (I) with the fg3 metadata, and field 5 (X) with an empty byte array (the
// presence bit is set even though the bytes are empty). Fields 1, 6 and 7 are
// left unset. The metadata reflects dc81.q with Argos enabled and an empty
// ARGOS_CONFIGURATION / routing tag; set request.Metadata entries explicitly
// for the device's actual configuration.
func NewLoginAfxRequest(path string) *AfxRequest {
	request := &AfxRequest{Metadata: &AfxMetadata{
		Entries: append([]*AfxMetadataEntry(nil), attestationMetadataEntries...),
	}}
	request.SetValue(path)
	request.SetType(AttestationTypeLogin)
	request.SetX(nil)
	return request
}

// Attestor produces the native attestation blob (the afx output that becomes
// fs60 field 11 / n070 field 15). The wire format of the afx *request* is
// implemented in AfxRequest; how the blob is produced is pluggable so the
// bridge can use either an on-device oracle or, later, a Go reimplementation
// of prh.a.
type Attestor interface {
	// Attest returns the attestation bytes for a serialized afx request.
	Attest(ctx context.Context, request []byte) ([]byte, error)
}

// AfxRequest mirrors ydx, the afx attestation-request message handed to
// yblj.prh.a([B)[B] in the 14.30.0.39 APK. Field names are the obfuscated
// MessageNano names; the wire tags below are the recovered writeTo order.
//
// MessageNano tracks presence with a bitmask (ydx.a): a field is serialized
// when its presence bit is set, even for empty strings or empty byte slices.
// The has* flags below reproduce that behavior so the bytes match the APK
// exactly.
type AfxRequest struct {
	Path     string       // field 1 (obfuscated "b"): set via ydx.e(String)
	Value    string       // field 2 (obfuscated "c"): set via ydx.d(String)
	Type     int32        // field 3 (obfuscated "t"): AttestationTypeLogin
	Metadata *AfxMetadata // field 4 (obfuscated "I"): fg3 metadata
	X        []byte       // field 5: set via ydx.a(byte[])
	Y        []byte       // field 6: set via ydx.c(byte[])
	Z        int64        // field 7: set by direct assignment

	hasPath  bool
	hasValue bool
	hasType  bool
	hasX     bool
	hasY     bool
	hasZ     bool
}

// SetPath records field 1 as present (mirrors ydx.e).
func (message *AfxRequest) SetPath(value string) {
	message.Path = value
	message.hasPath = true
}

// SetValue records field 2 as present (mirrors ydx.d).
func (message *AfxRequest) SetValue(value string) {
	message.Value = value
	message.hasValue = true
}

// SetType records field 3 as present (mirrors ydx.g).
func (message *AfxRequest) SetType(value int32) {
	message.Type = value
	message.hasType = true
}

// SetX records field 5 as present (mirrors ydx.a). An empty slice is still
// serialized, matching MessageNano.
func (message *AfxRequest) SetX(value []byte) {
	message.X = value
	message.hasX = true
}

// SetY records field 6 as present (mirrors ydx.c).
func (message *AfxRequest) SetY(value []byte) {
	message.Y = value
	message.hasY = true
}

// SetZ records field 7 as present, mirroring the ydx.Z presence bit.
func (message *AfxRequest) SetZ(value int64) {
	message.Z = value
	message.hasZ = true
}

// Marshal serializes ydx in writeTo order so the bytes match MessageNano
// output for the same inputs.
func (message *AfxRequest) Marshal() ([]byte, error) {
	if !utf8.ValidString(message.Path) || !utf8.ValidString(message.Value) {
		return nil, errors.New("afx path and value must be valid UTF-8")
	}
	var wire []byte
	if message.hasPath {
		wire = protowire.AppendTag(wire, 1, protowire.BytesType)
		wire = protowire.AppendString(wire, message.Path)
	}
	if message.hasValue {
		wire = protowire.AppendTag(wire, 2, protowire.BytesType)
		wire = protowire.AppendString(wire, message.Value)
	}
	if message.hasType {
		wire = protowire.AppendTag(wire, 3, protowire.VarintType)
		wire = protowire.AppendVarint(wire, uint64(message.Type))
	}
	if message.Metadata != nil {
		encoded, err := message.Metadata.Marshal()
		if err != nil {
			return nil, err
		}
		wire = protowire.AppendTag(wire, 4, protowire.BytesType)
		wire = protowire.AppendBytes(wire, encoded)
	}
	if message.hasX {
		wire = protowire.AppendTag(wire, 5, protowire.BytesType)
		wire = protowire.AppendBytes(wire, message.X)
	}
	if message.hasY {
		wire = protowire.AppendTag(wire, 6, protowire.BytesType)
		wire = protowire.AppendBytes(wire, message.Y)
	}
	if message.hasZ {
		wire = protowire.AppendTag(wire, 7, protowire.VarintType)
		wire = protowire.AppendVarint(wire, uint64(message.Z))
	}
	return wire, nil
}

// AfxMetadata mirrors fg3, the fg3 metadata message nested in AfxRequest field
// 4. Recovered writeTo order; field names are obfuscated. Fields 2, 3 and 5
// are booleans (not int32), so they carry their own presence bits.
type AfxMetadata struct {
	Entries []*AfxMetadataEntry // field 1 (obfuscated "b"), repeated
	C       bool                // field 2 (obfuscated "c")
	T       bool                // field 3 (obfuscated "t")
	I       int64               // field 4 (obfuscated "I"), int64
	X       bool                // field 5 (obfuscated "X")

	hasC bool
	hasT bool
	hasI bool
	hasX bool
}

// SetC records field 2 as present.
func (message *AfxMetadata) SetC(value bool) {
	message.C = value
	message.hasC = true
}

// SetT records field 3 as present.
func (message *AfxMetadata) SetT(value bool) {
	message.T = value
	message.hasT = true
}

// SetI records field 4 as present.
func (message *AfxMetadata) SetI(value int64) {
	message.I = value
	message.hasI = true
}

// SetX records field 5 as present.
func (message *AfxMetadata) SetX(value bool) {
	message.X = value
	message.hasX = true
}

// Marshal serializes fg3 in writeTo order.
func (message *AfxMetadata) Marshal() ([]byte, error) {
	var wire []byte
	for _, entry := range message.Entries {
		if entry == nil {
			continue
		}
		encoded, err := entry.Marshal()
		if err != nil {
			return nil, err
		}
		wire = protowire.AppendTag(wire, 1, protowire.BytesType)
		wire = protowire.AppendBytes(wire, encoded)
	}
	if message.hasC {
		wire = protowire.AppendTag(wire, 2, protowire.VarintType)
		wire = protowire.AppendVarint(wire, boolToVarint(message.C))
	}
	if message.hasT {
		wire = protowire.AppendTag(wire, 3, protowire.VarintType)
		wire = protowire.AppendVarint(wire, boolToVarint(message.T))
	}
	if message.hasI {
		wire = protowire.AppendTag(wire, 4, protowire.VarintType)
		wire = protowire.AppendVarint(wire, uint64(message.I))
	}
	if message.hasX {
		wire = protowire.AppendTag(wire, 5, protowire.VarintType)
		wire = protowire.AppendVarint(wire, boolToVarint(message.X))
	}
	return wire, nil
}

// AfxMetadataEntry mirrors kte, the repeated element inside fg3 field 1. Field
// 4 is a boolean (not int32), so it carries its own presence bit.
type AfxMetadataEntry struct {
	B []string // field 1, repeated string
	C []string // field 2, repeated string
	T int32    // field 3, enum 0..6
	I bool     // field 4

	hasT bool
	hasI bool
}

// SetT records field 3 as present.
func (message *AfxMetadataEntry) SetT(value int32) {
	message.T = value
	message.hasT = true
}

// SetI records field 4 as present.
func (message *AfxMetadataEntry) SetI(value bool) {
	message.I = value
	message.hasI = true
}

// Marshal serializes kte in writeTo order.
func (message *AfxMetadataEntry) Marshal() ([]byte, error) {
	var wire []byte
	for _, value := range message.B {
		wire = protowire.AppendTag(wire, 1, protowire.BytesType)
		wire = protowire.AppendString(wire, value)
	}
	for _, value := range message.C {
		wire = protowire.AppendTag(wire, 2, protowire.BytesType)
		wire = protowire.AppendString(wire, value)
	}
	if message.hasT {
		wire = protowire.AppendTag(wire, 3, protowire.VarintType)
		wire = protowire.AppendVarint(wire, uint64(message.T))
	}
	if message.hasI {
		wire = protowire.AppendTag(wire, 4, protowire.VarintType)
		wire = protowire.AppendVarint(wire, boolToVarint(message.I))
	}
	return wire, nil
}

func boolToVarint(value bool) uint64 {
	if value {
		return 1
	}
	return 0
}

// CommandAttestor is an Attestor that shells out to a local command to produce
// the attestation blob. The command receives the serialized afx request on
// stdin and must write the raw attestation bytes to stdout. This is the Option
// A oracle plumbing: point it at an on-device helper (e.g. via `adb shell`) or
// any future local reimplementation of prh.a.
type CommandAttestor struct {
	// Name is the executable (default "adb").
	Name string
	// Args are the executable arguments.
	Args []string
}

// Attest runs the configured command and returns its stdout as the blob.
func (attestor *CommandAttestor) Attest(ctx context.Context, request []byte) ([]byte, error) {
	name := attestor.Name
	if name == "" {
		name = "adb"
	}
	command := exec.CommandContext(ctx, name, attestor.Args...)
	command.Stdin = bytes.NewReader(request)
	output, err := command.Output()
	if err != nil {
		return nil, err
	}
	return output, nil
}
