module desktop-audio-ctrl

go 1.23.0

require (
	github.com/go-ole/go-ole v1.3.0
	github.com/karalabe/usb v0.0.2
	github.com/moutend/go-wca v0.3.0
	github.com/tarm/serial v0.0.0-20180830185346-98f6abe2eb07
	gopkg.in/yaml.v2 v2.4.0
	tinygo.org/x/drivers v0.29.0
	tinygo.org/x/tinyfont v0.3.0
)

require (
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	golang.org/x/sys v0.27.0 // indirect
)

replace github.com/moutend/go-wca => github.com/dikkadev/go-wca v0.0.0-20241130215409-f12e08875c45
