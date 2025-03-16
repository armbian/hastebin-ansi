package static

import "embed"

//go:embed *.html *.css *.js *.ico *.png *.txt
var StaticFS embed.FS
