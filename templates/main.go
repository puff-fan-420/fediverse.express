package templates

import _ "embed"

//go:embed header.html
var Header string

//go:embed footer.html
var Footer string

//go:embed index.html
var Index string

//go:embed provision.html
var Provision string

//go:embed verify.html
var Verify string

//go:embed install.html
var Install string

//go:embed running.html
var Running string

//go:embed done.html
var Done string

//go:embed prov.html
var Prov string

//go:embed contact.html
var Contact string
