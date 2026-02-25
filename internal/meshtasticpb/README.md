# Meshtastic protobuf generation

These files are generated from Meshtastic official protobuf schemas.

Pinned source:
- Repo: `https://github.com/meshtastic/protobufs`
- Commit: `27591d98c4ca58630ddcc8bd4d3055033873a56c`
- Commit date/subject: `2026-02-09 Merge pull request #859 from meshtastic/st31`

Generation command (requires `protoc` and `protoc-gen-go`):

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.10
PATH="$(go env GOPATH)/bin:$PATH" \
  protoc -I /path/to/protobufs \
  --go_out=/tmp/meshtastic-pb-out \
  /path/to/protobufs/nanopb.proto \
  /path/to/protobufs/meshtastic/*.proto
cp /tmp/meshtastic-pb-out/github.com/meshtastic/go/generated/*.pb.go internal/radio/meshtasticpb/
```
