package main

import (
	"log"
	"time"

	"github.com/karalabe/usb"
)

func main() {
	ctx := usb.NewContext()
	defer ctx.Close()

	// Replace with your device's VID and PID
	const vendorID = 0x1234
	const productID = 0x5678

	for {
		devices, err := ctx.ListDevices(func(desc *usb.Descriptor) bool {
			// return desc.Vendor == vendorID && desc.Product == productID
			return true
		})
		if err != nil {
			log.Printf("Error finding devices: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if len(devices) == 0 {
			log.Println("Device not found. Retrying...")
			time.Sleep(2 * time.Second)
			continue
		}

		for i, device := range devices {
			log.Printf("Device %d: %s", i, device)
		}

		device := devices[0]
		defer device.Close()

		log.Println("Device connected.")

		go func() {
			for {
				data := make([]byte, 5)
				_, err := device.Control(0xA1, 0x01, 0, 0, data)
				if err != nil {
					log.Printf("Error reading: %v", err)
					return
				}
				log.Printf("Received: %x", data)
			}
		}()

		for {
			data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
			err := device.ControlWrite(0x21, 0x09, 0x200, 0, data)
			if err != nil {
				log.Printf("Error writing: %v", err)
				break
			}
			time.Sleep(1 * time.Second)
		}
	}
}
