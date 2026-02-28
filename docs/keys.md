# Key Handling

This project accepts Meshtastic channel PSKs in the same shorthand form users
normally configure in Meshtastic clients.

## Default public channel

The default public Meshtastic channel is commonly shown as:

```text
AQ==
```

That value is not the final AES key. It is a one-byte Meshtastic shorthand:

- `AQ==` base64-decodes to `{0x01}`
- Meshtastic defines `{0x01}` as "use the default public channel key"
- the parser expands that shorthand to the real 16-byte AES-128 key before
  attempting decryption

In code, that expanded key lives in
[`internal/meshtastic/crypto.go`](../internal/meshtastic/crypto.go)
as `defaultChannelKeyExpandedBytes`.
See [`ChannelSettings.psk` in Meshtastic's protobuf definition](https://git.skobk.in/skobkin/meshmap-lite/src/commit/d75fe7cc11584d40794246da04af37dd047df261/internal/meshtasticpb/channel.pb.go#L109-L119)
for more details.

## Why expansion is needed

AES cannot decrypt with a one-byte key. Meshtastic uses special shorthand PSK
values for user convenience, and those values must be expanded into the actual
AES key material used on the wire.

This matters for the default public channel and for related shorthand values
that Meshtastic derives from the same default key.

## How configuration interacts with this

The application config still treats `AQ==` as the default channel PSK string.
That default is applied in
[`internal/config/config.go`](../internal/config/config.go).

At ingest time:

1. configured channel PSK strings are passed into `meshtastic.ConfigureChannelKeys`
2. the parser decodes the configured PSK
3. if it is a one-byte shorthand, the parser expands it before decrypting
4. if it is already a 16-byte or 32-byte PSK, that configured key is used directly

So a configured non-default PSK overrides the default behavior as expected.
