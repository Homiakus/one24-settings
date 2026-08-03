module modbus-configurator

go 1.26.0

require (
	github.com/BurntSushi/toml v1.6.0
	github.com/goburrow/modbus v0.1.0
	github.com/gorilla/websocket v1.5.3
	golang.org/x/time v0.15.0
)

require github.com/goburrow/serial v0.1.0 // indirect

replace github.com/goburrow/serial => ../../../11_Software/Main-v6/third_party/goburrow-serial
