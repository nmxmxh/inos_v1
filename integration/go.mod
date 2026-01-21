module github.com/nmxmxh/inos_v1/integration

go 1.24.0

require (
	github.com/nmxmxh/inos_v1/kernel v0.0.0
	github.com/stretchr/testify v1.9.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	zombiezen.com/go/capnproto2 v2.18.2+incompatible // indirect
)

replace github.com/nmxmxh/inos_v1/kernel => ../kernel
