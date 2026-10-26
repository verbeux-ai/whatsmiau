module github.com/verbeux-ai/whatsmiau

go 1.26.0

require (
	cloud.google.com/go/logging v1.13.1
	cloud.google.com/go/storage v1.56.0
	github.com/XSAM/otelsql v0.44.0
	github.com/alicebob/miniredis/v2 v2.35.0
	github.com/caarlos0/env/v11 v11.3.1
	github.com/coder/websocket v1.8.15
	github.com/emersion/go-vcard v0.0.0-20241024213814-c9703dde27ff
	github.com/go-playground/validator/v10 v10.26.0
	github.com/go-redis/redis/v8 v8.11.5
	github.com/google/uuid v1.6.0
	github.com/joho/godotenv v1.5.1
	github.com/labstack/echo/v4 v4.13.4
	github.com/lib/pq v1.10.9
	github.com/mattn/go-sqlite3 v1.14.52
	github.com/purpshell/meowcaller v0.0.0-20260724092103-1ac278769bdc
	github.com/puzpuzpuz/xsync/v4 v4.1.0
	github.com/rs/zerolog v1.35.1
	github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e
	github.com/stretchr/testify v1.12.1
	github.com/swaggo/echo-swagger v1.5.2
	github.com/swaggo/swag v1.16.6
	github.com/urfave/cli/v2 v2.25.1
	go.mau.fi/whatsmeow v0.0.0-20260609091626-4e622162b959
	go.opentelemetry.io/otel v1.47.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.47.0
	go.opentelemetry.io/otel/sdk v1.47.0
	go.opentelemetry.io/otel/trace v1.47.0
	go.uber.org/zap v1.27.0
	golang.org/x/image v0.45.0
	golang.org/x/net v0.61.0
	golang.org/x/oauth2 v0.36.0
	golang.org/x/sync v0.24.0
	google.golang.org/api v0.264.0
	google.golang.org/protobuf v1.36.12
	sigs.k8s.io/yaml v1.3.0
)

require (
	cel.dev/expr v0.25.2 // indirect
	cloud.google.com/go v0.123.0 // indirect
	cloud.google.com/go/auth v0.18.2 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	cloud.google.com/go/iam v1.5.3 // indirect
	cloud.google.com/go/longrunning v0.8.0 // indirect
	cloud.google.com/go/monitoring v1.24.3 // indirect
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.33.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.53.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.53.0 // indirect
	github.com/KyleBanks/depth v1.2.1 // indirect
	github.com/beeper/argo-go v1.1.2 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cncf/xds/go v0.0.0-20260202195803-dba9d589def2 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/elliotchance/orderedmap/v3 v3.1.0 // indirect
	github.com/envoyproxy/go-control-plane/envoy v1.37.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.3.3 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.8 // indirect
	github.com/go-jose/go-jose/v4 v4.1.4 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v1.0.0 // indirect
	github.com/go-openapi/jsonreference v1.0.0 // indirect
	github.com/go-openapi/spec v0.22.9 // indirect
	github.com/go-openapi/swag/conv v0.28.0 // indirect
	github.com/go-openapi/swag/jsonutils v0.28.0 // indirect
	github.com/go-openapi/swag/loading v0.28.0 // indirect
	github.com/go-openapi/swag/pools v0.28.0 // indirect
	github.com/go-openapi/swag/stringutils v0.28.0 // indirect
	github.com/go-openapi/swag/typeutils v0.28.0 // indirect
	github.com/go-openapi/swag/yamlutils v0.28.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.11 // indirect
	github.com/googleapis/gax-go/v2 v2.17.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.30.0 // indirect
	github.com/hajimehoshi/go-mp3 v0.3.4 // indirect
	github.com/labstack/gommon v0.4.2 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/petermattis/goid v0.0.0-20260918085751-abfca077860b // indirect
	github.com/pion/datachannel v1.6.2 // indirect
	github.com/pion/dtls/v3 v3.1.5 // indirect
	github.com/pion/logging v0.2.4 // indirect
	github.com/pion/opus v0.1.0 // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pion/sctp v1.11.0 // indirect
	github.com/pion/transport/v4 v4.0.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/spiffe/go-spiffe/v2 v2.7.0 // indirect
	github.com/sv-tools/openapi v0.2.1 // indirect
	github.com/swaggo/files/v2 v2.0.0 // indirect
	github.com/swaggo/swag/v2 v2.0.0-rc4 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	github.com/vektah/gqlparser/v2 v2.5.27 // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	go.mau.fi/libsignal v0.2.2 // indirect
	go.mau.fi/util v0.11.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/detectors/gcp v1.44.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.70.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.70.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.47.0 // indirect
	go.opentelemetry.io/otel/log v1.47.0 // indirect
	go.opentelemetry.io/otel/metric v1.47.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.47.0 // indirect
	go.opentelemetry.io/proto/otlp v1.11.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/crypto v0.58.0 // indirect
	golang.org/x/exp v0.0.0-20261014143641-f0251eb4aa3e // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	golang.org/x/tools v0.52.0 // indirect
	google.golang.org/genproto v0.0.0-20260128011058-8636f8732409 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260928230214-8a89bd6388cc // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260928230214-8a89bd6388cc // indirect
	google.golang.org/grpc v1.83.2 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace go.mau.fi/whatsmeow => github.com/verbeux-ai/whatsmeow v1.3.7
