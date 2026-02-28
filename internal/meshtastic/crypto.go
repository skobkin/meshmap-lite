package meshtastic

import (
	cryptoaes "crypto/aes"
	cryptocipher "crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"

	generated "meshmap-lite/internal/meshtasticpb"

	"google.golang.org/protobuf/proto"
)

var (
	// Meshtastic treats one-byte PSKs as shorthand values. "AQ==" decodes to
	// {0x01}, which expands to this default public channel AES-128 key.
	// See the generated protobuf comment in internal/meshtasticpb/channel.pb.go.
	defaultChannelKeyExpandedBytes = [16]byte{
		0xd4, 0xf1, 0xbb, 0x3a, 0x20, 0x29, 0x07, 0x59,
		0xf0, 0xbc, 0xff, 0xab, 0xcf, 0x4e, 0x69, 0x01,
	}

	channelKeyMu sync.RWMutex
	channelKeys  = map[string]string{}
)

// ConfigureChannelKeys sets channel-name->PSK mappings used for decrypting
// MQTT encrypted MeshPacket payloads.
func ConfigureChannelKeys(m map[string]string) {
	next := make(map[string]string, len(m))
	for k, v := range m {
		name := strings.TrimSpace(k)
		if name == "" {
			continue
		}
		next[name] = strings.TrimSpace(v)
	}

	channelKeyMu.Lock()
	channelKeys = next
	channelKeyMu.Unlock()
}

func decryptPacket(packet *generated.MeshPacket, envelopeChannelID, topicChannel string) (*generated.Data, error) {
	if packet.GetPkiEncrypted() {
		return nil, fmt.Errorf("pki encrypted packet unsupported in mqtt parser")
	}

	ciphertext := packet.GetEncrypted()
	if len(ciphertext) == 0 {
		return nil, fmt.Errorf("missing encrypted payload")
	}

	keys := configuredChannelKeys()
	candidates := buildChannelCandidates(keys, envelopeChannelID, topicChannel)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no channel key configured for encrypted packet")
	}

	hash := byte(packet.GetChannel() & 0xff)
	tried := 0
	for _, candidate := range candidates {
		if candidate.KeyLen <= 0 || candidate.Hash != hash {
			continue
		}
		tried++
		if data, ok := tryDecryptData(packet, ciphertext, candidate.Key[:candidate.KeyLen]); ok {
			return data, nil
		}
	}

	// Hash is a hint; if no exact-hash candidate works, try remaining keys.
	for _, candidate := range candidates {
		if candidate.KeyLen <= 0 || candidate.Hash == hash {
			continue
		}
		tried++
		if data, ok := tryDecryptData(packet, ciphertext, candidate.Key[:candidate.KeyLen]); ok {
			return data, nil
		}
	}
	if tried == 0 {
		return nil, fmt.Errorf("no decryptable keys for encrypted packet")
	}

	return nil, fmt.Errorf("failed to decrypt encrypted packet (bad psk?)")
}

func decryptPacketIfPossible(packet *generated.MeshPacket, envelopeChannelID, topicChannel string) (*generated.Data, bool) {
	decoded, err := decryptPacket(packet, envelopeChannelID, topicChannel)
	if err != nil {
		return nil, false
	}

	return decoded, true
}

type channelCandidate struct {
	Name   string
	Key    [32]byte
	KeyLen int
	Hash   byte
}

func buildChannelCandidates(keys map[string]string, envelopeChannelID, topicChannel string) []channelCandidate {
	names := make([]string, 0, len(keys)+2)
	seen := map[string]struct{}{}
	addName := func(name string) {
		raw := strings.TrimSpace(name)
		if raw == "" {
			return
		}
		if _, ok := seen[raw]; ok {
			return
		}
		seen[raw] = struct{}{}
		names = append(names, raw)
	}

	addName(envelopeChannelID)
	addName(topicChannel)
	for key := range keys {
		addName(key)
	}

	out := make([]channelCandidate, 0, len(names))
	for _, name := range names {
		pskRaw, ok := keyForChannelName(keys, name)
		if !ok {
			continue
		}
		key, keyLen, ok := decodeAndExpandPSK(pskRaw)
		if !ok {
			continue
		}
		out = append(out, channelCandidate{
			Name:   name,
			Key:    key,
			KeyLen: keyLen,
			Hash:   channelHash(name, key[:keyLen]),
		})
	}

	return out
}

func keyForChannelName(keys map[string]string, name string) (string, bool) {
	needle := strings.TrimSpace(name)
	psk, ok := keys[needle]
	if ok {
		return psk, true
	}

	for key, value := range keys {
		if strings.EqualFold(strings.TrimSpace(key), needle) {
			return value, true
		}
	}

	return "", false
}

func decodeAndExpandPSK(encoded string) ([32]byte, int, bool) {
	var out [32]byte
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return out, 0, false
	}

	keyBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		keyBytes, err = base64.RawStdEncoding.DecodeString(encoded)
		if err != nil {
			return out, 0, false
		}
	}

	switch keyLen := len(keyBytes); {
	case keyLen == 0:
		return out, 0, true
	case keyLen == 1:
		idx := keyBytes[0]
		if idx == 0 {
			return out, 0, true
		}
		copy(out[:], defaultChannelKeyExpandedBytes[:])
		out[len(defaultChannelKeyExpandedBytes)-1] = out[len(defaultChannelKeyExpandedBytes)-1] + idx - 1

		return out, len(defaultChannelKeyExpandedBytes), true
	case keyLen < 16:
		copy(out[:], keyBytes)

		return out, 16, true
	case keyLen == 16:
		copy(out[:], keyBytes)

		return out, 16, true
	case keyLen < 32:
		copy(out[:], keyBytes)

		return out, 32, true
	case keyLen == 32:
		copy(out[:], keyBytes)

		return out, 32, true
	default:
		return out, 0, false
	}
}

func configuredChannelKeys() map[string]string {
	channelKeyMu.RLock()
	defer channelKeyMu.RUnlock()

	out := make(map[string]string, len(channelKeys))
	for key, value := range channelKeys {
		out[key] = value
	}

	return out
}

func tryDecryptData(packet *generated.MeshPacket, ciphertext, key []byte) (*generated.Data, bool) {
	block, err := cryptoaes.NewCipher(key)
	if err != nil {
		return nil, false
	}

	iv := make([]byte, 16)
	binary.LittleEndian.PutUint64(iv[0:8], uint64(packet.GetId()))
	binary.LittleEndian.PutUint32(iv[8:12], packet.GetFrom())

	plaintext := make([]byte, len(ciphertext))
	copy(plaintext, ciphertext)
	cryptocipher.NewCTR(block, iv).XORKeyStream(plaintext, plaintext)

	var data generated.Data
	if err := proto.Unmarshal(plaintext, &data); err != nil {
		return nil, false
	}
	if data.GetPortnum() == generated.PortNum_UNKNOWN_APP {
		return nil, false
	}

	return &data, true
}
func channelHash(name string, key []byte) byte {
	var hash byte
	for i := 0; i < len(name); i++ {
		hash ^= name[i]
	}
	for i := 0; i < len(key); i++ {
		hash ^= key[i]
	}

	return hash
}
