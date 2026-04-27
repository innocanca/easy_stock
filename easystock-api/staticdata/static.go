package staticdata

import _ "embed"

//go:embed dataset.json
var DatasetJSON []byte
