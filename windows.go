package main

import (
	"bufio"
	"desktop-audio-ctrl/protocol"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
	"github.com/tarm/serial"
	"gopkg.in/yaml.v2"
)

type ComboConfig struct {
	Combo    uint8  `yaml:"combo"`
	DeviceID string `yaml:"deviceID"`
}

type Config struct {
	PortName           string        `yaml:"portName"`
	BaudRate           int           `yaml:"baudRate"`
	Combos             []ComboConfig `yaml:"combos"`
	ConfigReloadPeriod time.Duration `yaml:"configReloadPeriod"`
	SetEventPeriod     time.Duration `yaml:"setEventPeriod"` // New field
	LogFile            string        `yaml:"logFile"`
}

var (
	config     Config
	configFile = "config.yaml"
	configLock sync.RWMutex
	oleLock    sync.Mutex

	mmde     *wca.IMMDeviceEnumerator
	mmdeOnce sync.Once

	writeChan    = make(chan protocol.Event, 100) // Buffered channel for outgoing writes
	eventChan    = make(chan protocol.Event, 100) // Buffered channel for incoming events
	shutdownChan = make(chan struct{})            // Channel to signal shutdown
)

func initLogging(logFile string) {
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal("Error opening log file:", err)
	}
	log.SetOutput(file)
	log.Println("Logging initialized.")
}

func loadConfig() {
	configLock.Lock()
	defer configLock.Unlock()

	data, err := os.ReadFile(configFile)
	if err != nil {
		log.Println("Error reading config file:", err)
		return
	}

	var newConfig Config
	err = yaml.Unmarshal(data, &newConfig)
	if err != nil {
		log.Println("Error parsing config file:", err)
		return
	}

	config = newConfig
	log.Println("Configuration reloaded.")
}

func getComboConfig(combo uint8) *ComboConfig {
	configLock.RLock()
	defer configLock.RUnlock()

	for _, c := range config.Combos {
		if c.Combo == combo {
			return &c
		}
	}
	return nil
}

func getCurrentVolume(deviceID string) (int, error) {
	vol, err := oleInvoke(deviceID, func(aev *wca.IAudioEndpointVolume) (interface{}, error) {
		var level float32
		err := aev.GetMasterVolumeLevelScalar(&level)
		if err != nil {
			return nil, err
		}
		vol := int(level * 100.0)
		return vol, nil
	})
	if err != nil {
		return 0, err
	}
	return vol.(int), nil
}

func setVolume(deviceID string, volumeLevel float32) error {
	_, err := oleInvoke(deviceID, func(aev *wca.IAudioEndpointVolume) (interface{}, error) {
		err := aev.SetMasterVolumeLevelScalar(volumeLevel, nil)
		if err != nil {
			return nil, fmt.Errorf("SetMasterVolumeLevelScalar failed: %w", err)
		}
		return nil, nil
	})
	return err
}

func getDeviceEnumerator() (*wca.IMMDeviceEnumerator, error) {
	var err error
	mmdeOnce.Do(func() {
		err = wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &mmde)
		if err != nil {
			log.Println("Failed to create IMMDeviceEnumerator:", err)
		}
	})
	return mmde, err
}

func oleInvoke(deviceID string, f func(aev *wca.IAudioEndpointVolume) (interface{}, error)) (interface{}, error) {
	oleLock.Lock()
	defer oleLock.Unlock()

	mmde, err := getDeviceEnumerator()
	if err != nil {
		return nil, err
	}

	var mmd *wca.IMMDevice
	if err = mmde.GetDevice(deviceID, &mmd); err != nil {
		return nil, fmt.Errorf("GetDevice failed: %w", err)
	}
	defer mmd.Release()

	var ps *wca.IPropertyStore
	if err = mmd.OpenPropertyStore(wca.STGM_READ, &ps); err != nil {
		return nil, fmt.Errorf("OpenPropertyStore failed: %w", err)
	}
	defer ps.Release()

	var aev *wca.IAudioEndpointVolume
	if err = mmd.Activate(wca.IID_IAudioEndpointVolume, wca.CLSCTX_ALL, nil, &aev); err != nil {
		return nil, fmt.Errorf("Activate IAudioEndpointVolume failed: %w", err)
	}
	defer aev.Release()

	return f(aev)
}

func handleEvent(event protocol.Event) {
	log.Println("Received event:", event.String())

	comboConfig := getComboConfig(event.Combo)
	if comboConfig == nil {
		log.Println("No configuration found for combo:", event.Combo)
		return
	}

	state := event.State
	if state < 0 {
		state = 0
	} else if state > 100 {
		state = 100
	}

	volumeLevel := float32(state) / 100.0

	err := setVolume(comboConfig.DeviceID, volumeLevel)
	if err != nil {
		log.Println("Error setting volume for device", comboConfig.DeviceID, ":", err)
	} else {
		log.Printf("Set volume to %d%% for device %s", state, comboConfig.DeviceID)
	}
}

func configReloader(shutdownChan <-chan struct{}) {
	configLock.RLock()
	period := config.ConfigReloadPeriod
	configLock.RUnlock()

	ticker := time.NewTicker(period)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			loadConfig()
		case <-shutdownChan:
			log.Println("Configuration reloader shutting down.")
			return
		}
	}
}

func marshalEvent(event protocol.Event) []byte {
	return protocol.Marshal(event)
}

func serialHandler(
	s *serial.Port,
	writeChan <-chan protocol.Event,
	eventChan chan<- protocol.Event,
	shutdownChan <-chan struct{},
) {
	// Initialize OLE for this goroutine
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	if err != nil {
		log.Fatalf("Failed to initialize COM in serialHandler: %v", err)
	}
	defer ole.CoUninitialize()

	reader := bufio.NewReader(s)
	buffer := make([]byte, 0, 5)
	const (
		maxConsecutiveErrors = 10
		backoffDuration      = 1 * time.Second
	)
	consecutiveErrors := 0

	for {
		select {
		case <-shutdownChan:
			log.Println("Serial handler shutting down.")
			return
		default:
			// Attempt to read a byte
			byteRead, err := reader.ReadByte()
			if err != nil {
				if err == io.EOF {
					log.Println("EOF received, serial handler shutting down.")
					return
				}
				log.Printf("Error reading from serial port: %v", err)
				consecutiveErrors++
				if consecutiveErrors >= maxConsecutiveErrors {
					log.Printf("Exceeded maximum consecutive errors (%d). Shutting down serial handler.", maxConsecutiveErrors)
					return
				}
				time.Sleep(backoffDuration) // Prevent tight loop on errors
				continue
			}

			// Reset error count on successful read
			consecutiveErrors = 0

			buffer = append(buffer, byteRead)
			// Process buffer for complete packets (assuming 5-byte packets)
			for len(buffer) >= 5 {
				packet := buffer[:5]
				event, ok := protocol.Unmarshal(packet)
				if ok {
					select {
					case eventChan <- event:
						// Event sent successfully
					default:
						log.Println("Warning: eventChan is full, dropping event.")
					}
					buffer = buffer[5:]
				} else {
					log.Println("Invalid packet received, shifting buffer by 1 byte.")
					buffer = buffer[1:]
				}
			}

			// Handle outgoing writes if available
			select {
			case ev := <-writeChan:
				data := protocol.Marshal(ev)
				_, err := s.Write(data)
				if err != nil {
					log.Println("Error writing to serial port:", err)
				} else {
					log.Printf("Sent data: % X\n", data)
				}
			default:
				// No write to perform
			}
		}
	}
}

func setEventSender(writeChan chan<- protocol.Event, shutdownChan <-chan struct{}) {
	// Initialize OLE for this goroutine
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	if err != nil {
		log.Fatalf("Failed to initialize COM in setEventSender: %v", err)
	}
	defer ole.CoUninitialize()

	sendSetEvents := func() {
		log.Println("Sending set events to synchronize device state.")
		configLock.RLock()
		combos := config.Combos
		configLock.RUnlock()

		for _, combo := range combos {
			// Retrieve the current volume level
			currentVolume, err := getCurrentVolume(combo.DeviceID)
			if err != nil {
				log.Printf("Error getting current volume for device %s: %v", combo.DeviceID, err)
				continue
			}

			// Create a set event
			event := protocol.Event{
				Type:  protocol.EVENT_TYPE_SET,
				Combo: combo.Combo,
				State: uint8(currentVolume),
			}

			// Send the packet to writeChan
			select {
			case writeChan <- event:
				log.Printf("Queued set event for combo %d with state %d%%.", event.Combo, event.State)
			case <-shutdownChan:
				log.Println("Set event sender received shutdown signal.")
				return
			}
		}
	}

	// Initial synchronization at startup
	sendSetEvents()

	// Periodic synchronization based on SetEventPeriod
	configLock.RLock()
	period := config.SetEventPeriod
	configLock.RUnlock()

	ticker := time.NewTicker(period)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sendSetEvents()
		case <-shutdownChan:
			log.Println("Set event sender shutting down.")
			return
		}
	}
}

func eventProcessor(eventChan <-chan protocol.Event, shutdownChan <-chan struct{}) {
	for {
		select {
		case event := <-eventChan:
			handleEvent(event)
		case <-shutdownChan:
			log.Println("Event processor shutting down.")
			return
		}
	}
}

func main() {
	portName := flag.String("port", "", "Serial port name (e.g., COM3)")
	flag.Parse()

	loadConfig()

	if *portName != "" {
		configLock.Lock()
		config.PortName = *portName
		configLock.Unlock()
	}

	if config.PortName == "" {
		log.Fatal("No serial port specified. Use the -port flag to specify the serial port.")
	}

	initLogging(config.LogFile)

	go configReloader(shutdownChan)

	// Open serial port with ReadTimeout set in serial.Config
	serialConfig := &serial.Config{
		Name:        config.PortName,
		Baud:        config.BaudRate,
		ReadTimeout: config.ConfigReloadPeriod, // Adjust as needed
	}
	s, err := serial.OpenPort(serialConfig)
	if err != nil {
		log.Fatalf("Failed to open serial port %s: %v", config.PortName, err)
	}
	defer func() {
		err := s.Close()
		if err != nil {
			log.Println("Error closing serial port:", err)
		}
	}()
	log.Printf("Serial port %s opened with baud rate %d.", config.PortName, config.BaudRate)

	// greeting := []byte("Moin\n")
	// _, err = s.Write(greeting)
	// if err != nil {
	// 	log.Fatalf("Failed to send greeting to serial port: %v", err)
	// }
	// log.Printf("Sent greeting: %s", greeting)

	// Start serial handler goroutine
	go serialHandler(s, writeChan, eventChan, shutdownChan)

	// Start set event sender goroutine
	go setEventSender(writeChan, shutdownChan)

	// Start event processor goroutine
	go eventProcessor(eventChan, shutdownChan)

	// Wait for interrupt signal to gracefully shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	log.Println("Application is running. Press Ctrl+C to exit.")
	<-sigs
	log.Println("Interrupt signal received. Initiating shutdown.")

	// Signal all goroutines to stop
	close(shutdownChan)

	// Allow some time for goroutines to finish
	time.Sleep(1 * time.Second)
	log.Println("Application terminated gracefully.")
}
