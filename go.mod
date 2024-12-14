module desktop-audio-ctrl

go 1.23.0

require (
	github.com/dikkadev/prettyslog v0.0.0-20241029122445-44f60ae978bd
	github.com/go-ole/go-ole v1.3.0
	github.com/karalabe/usb v0.0.2
	github.com/moutend/go-wca v0.3.0
	go.bug.st/serial v1.6.2
	gopkg.in/yaml.v2 v2.4.0
	tinygo.org/x/drivers v0.29.0
	tinygo.org/x/tinyfont v0.3.0
)

require (
	github.com/creack/goselect v0.1.2 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	golang.org/x/sys v0.28.0 // indirect
)

replace github.com/moutend/go-wca => github.com/dikkadev/go-wca v0.0.0-20241130215409-f12e08875c45
