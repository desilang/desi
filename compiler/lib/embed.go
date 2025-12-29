package lib

import (
	"embed"
)

//go:embed *.desi */__mod.desi */*.desi
var StdlibFS embed.FS
