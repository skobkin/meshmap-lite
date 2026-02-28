# Gathering real-world data

## MQTT

`mosquitto_sub` example with hex-encoded payloads:
```shell
mosquitto_sub -h mqtt.domain.tld -u username -P 'password' -t 'msh/RU/ARKH/#' -F '%I\t%t\t%x'
```

Replace `mqtt.domain.tld`, `username` and `password` with your MQTT server credentials.

## Local decode helper

For ad-hoc packet inspection, use the debug-only CLI:
```shell
go run -tags debugtools ./cmd/meshtastic-debug \
  -root-topic msh/RU/ARKH \
  -channel-key LongFast=AQ== \
  -topic 'msh/RU/ARKH/2/e/LongFast/!9028d008' \
  -payload-hex '0a7b...'
```

It also accepts `mosquitto_sub -F '%I\t%t\t%x'` lines from stdin:
```shell
mosquitto_sub -h mqtt.domain.tld -u username -P 'password' -t 'msh/RU/ARKH/#' -F '%I\t%t\t%x' | \
  go run -tags debugtools ./cmd/meshtastic-debug \
    -root-topic msh/RU/ARKH \
    -channel-key LongFast=AQ==
```

## Examples used for testing

Look at [`internal/meshtastic/testdata`](../internal/meshtastic/testdata) for gathered examples.
