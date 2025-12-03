# protoc-gen-plain-go

This is a protoc gen plugin to create plain Go structs with no imports.
This is needed to keep the WASM plugins as small as possible without the need
for external dependencies introduced by the `proto` package.
