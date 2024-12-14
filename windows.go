package main

import (
	"desktop-audio-ctrl/pkg/reliableserial"
	"desktop-audio-ctrl/protocol"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/dikkadev/prettyslog"
	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
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
	SetEventPeriod     time.Duration `yaml:"setEventPeriod"`
}

var (
	config     Config
	configFile = "config.yaml"
	configLock sync.RWMutex
	oleLock    sync.Mutex

	mmde     *wca.IMMDeviceEnumerator
	mmdeOnce sync.Once

	writeChan    = make(chan protocol.Event, 100)
	eventChan    = make(chan protocol.Event, 100)
	shutdownChan = make(chan struct{})
)

func loadConfig() {
	configLock.Lock()
	defer configLock.Unlock()

	data, err := os.ReadFile(configFile)
	if err != nil {
		// log.Println("Error reading config file:", err)
		slog.Warn("error reading config file", "err", err)
		return
	}

	var newConfig Config
	err = yaml.Unmarshal(data, &newConfig)
	if err != nil {
		// log.Println("Error parsing config file:", err)
		slog.Warn("error parsing config file", "err", err)
		return
	}

	config = newConfig
	// log.Println("Configuration reloaded.")
	slog.Info("configuration reloaded")
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
			// log.Println("Failed to create IMMDeviceEnumerator:", err)
			slog.Error("failed to create IMMDeviceEnumerator", "err", err)
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
	// log.Println("Received event:", event.String())
	slog.Info("received event", "event", event.String())

	comboConfig := getComboConfig(event.Combo)
	if comboConfig == nil {
		// log.Println("No configuration found for combo:", event.Combo)
		slog.Warn("no configuration found for combo", "combo", event.Combo)
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
		// log.Println("Error setting volume for device", comboConfig.DeviceID, ":", err)
		slog.Error("error setting volume", "deviceID", comboConfig.DeviceID, "err", err)
	} else {
		// log.Printf("Set volume to %d%% for device %s", state, comboConfig.DeviceID)
		slog.Info("set volume", "state", state, "deviceID", comboConfig.DeviceID)
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
			// log.Println("Configuration reloader shutting down.")
			slog.Info("configuration reloader shutting down")
			return
		}
	}
}

func setEventSender(writeChan chan<- reliableserial.Serializable, shutdownChan <-chan struct{}) {
	// Initialize OLE for this goroutine
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	if err != nil {
		log.Fatalf("Failed to initialize COM in setEventSender: %v", err)
	}
	defer ole.CoUninitialize()

	sendSetEvents := func() {
		// log.Println("Sending set events to synchronize device state.")
		slog.Info("sending set events to synchronize device state")
		configLock.RLock()
		combos := config.Combos
		configLock.RUnlock()

		for _, combo := range combos {
			// Retrieve the current volume level
			currentVolume, err := getCurrentVolume(combo.DeviceID)
			if err != nil {
				// log.Printf("Error getting current volume for device %s: %v", combo.DeviceID, err)
				slog.Error("error getting current volume", "deviceID", combo.DeviceID, "err", err)
				continue
			}

			// Create a set event
			event := &protocol.Event{
				Type:  protocol.EVENT_TYPE_SET,
				Combo: combo.Combo,
				State: uint8(currentVolume),
			}

			// Send the packet to writeChan
			select {
			case writeChan <- event:
				// log.Printf("Queued set event for combo %d with state %d%%.", event.Combo, event.State)
			case <-shutdownChan:
				// log.Println("Set event sender received shutdown signal.")
				slog.Info("set event sender received shutdown signal")
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
			// log.Println("Set event sender shutting down.")
			slog.Info("set event sender shutting down")
			return
		}
	}
}

type DeviceMatcher struct{}

func (DeviceMatcher) Match(info reliableserial.DeviceInfo) (_ bool) {
	return info.Name == config.PortName
}

func main() {
	logger := slog.New(prettyslog.NewPrettyslogHandler("5ac",
		prettyslog.WithLevel(slog.LevelDebug),
		// prettyslog.WithWriter(file),
	))

	slog.SetDefault(logger)
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

	// initLogging(config.LogFile)

	go configReloader(shutdownChan)

	rs := reliableserial.NewReliableSerial(
		DeviceMatcher{},
		reliableserial.SerialConfig{
			BaudRate: config.BaudRate,
		},
		logger,
		func() []byte {
			return []byte{0xF0}
		},
		func() reliableserial.Serializable { return &protocol.Event{} },
	)
	defer rs.Close()

	go setEventSender(rs.SendChannel(), shutdownChan)

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
		if err != nil {
			log.Fatalf("Failed to initialize COM in setEventSender: %v", err)
		}
		defer ole.CoUninitialize()

		for msg := range rs.ReceiveChannel() {
			if m, ok := msg.(*protocol.Event); ok {
				handleEvent(*m)
			}
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	// log.Println("Application is running. Press Ctrl+C to exit.")
	slog.Info("application is running. press Ctrl+C to exit")
	<-sigs
	// log.Println("Interrupt signal received. Initiating shutdown.")
	slog.Info("interrupt signal received. initiating shutdown")

	// Signal all goroutines to stop
	close(shutdownChan)

	// Allow some time for goroutines to finish
	time.Sleep(1 * time.Second)
	// log.Println("Application terminated gracefully.")
	slog.Info("application terminated gracefully")
}
