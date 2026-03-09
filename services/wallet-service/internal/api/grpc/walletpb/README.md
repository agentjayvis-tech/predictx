# Generated gRPC Code

This directory contains auto-generated Go code from the proto definition.

## Regenerating

Run this from the `services/wallet-service/` directory after installing protoc:

```bash
# macOS
brew install protobuf

# Ubuntu/Debian
sudo apt-get install protobuf-compiler

# Generate Go code
protoc --go_out=. --go-grpc_out=. proto/wallet.proto
```

This will create:
- `wallet.pb.go` — Message definitions
- `wallet_grpc.pb.go` — Service interface and client/server stubs

Do NOT edit these files manually. They are auto-generated and will be overwritten.
