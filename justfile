buf-update:
	cd proto && buf dep update

buf-gen:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -z "$(find proto/apis -name '*.proto' 2>/dev/null)" ]; then
        echo "no protos to generate"
        exit 0
    fi
    buf generate --template buf.gen.go.yaml
