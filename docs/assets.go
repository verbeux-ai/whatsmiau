package docs

import _ "embed"

// SwaggerJSON and SwaggerYAML are embedded so the generated contracts are
// available from the compiled binary and not only from the repository checkout.
//
//go:embed swagger.json
var SwaggerJSON []byte

//go:embed swagger.yaml
var SwaggerYAML []byte
