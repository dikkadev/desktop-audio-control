package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dikkadev/prettyslog"
	ole "github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"

	"log/slog"

	"flag"

	"desktop-audio-ctrl/pkg/reliableserial"
	"desktop-audio-ctrl/protocol"

	"gopkg.in/yaml.v2"
)

// AudioDevice represents a discovered audio device
type AudioDevice struct {
	ID   string
	Name string
}

type ComboConfig struct {
	Combo       uint8    `yaml:"combo"`
	DeviceName  string   `yaml:"deviceName"`           // Primary device name pattern
	FallbackNames []string `yaml:"fallbackNames,omitempty"` // Fallback device name patterns
	DeviceID    string   `yaml:"deviceID,omitempty"`   // Keep for backward compatibility
}

type Config struct {
	PortName           string        `yaml:"portName"`
	BaudRate           int           `yaml:"baudRate"`
	Combos             []ComboConfig `yaml:"combos"`
	ConfigReloadPeriod time.Duration `yaml:"configReloadPeriod"`
	SetEventPeriod     time.Duration `yaml:"setEventPeriod"`
	// DeviceMatchingVocabulary defines the ordered list of keywords used for 
	// semantic device matching. Order matters as it affects embedding dimensions.
	// Each keyword becomes a dimension in the feature vector used for similarity calculations.
	DeviceMatchingVocabulary []string `yaml:"deviceMatchingVocabulary,omitempty"`
}

var (
	config     Config
	configFile = "config.yaml"
	configLock sync.RWMutex
	oleLock    sync.Mutex

	mmde     *wca.IMMDeviceEnumerator
	mmdeOnce sync.Once

	// Cache for discovered devices
	deviceCache     []AudioDevice
	deviceCacheLock sync.RWMutex
	lastDeviceUpdate time.Time

	writeChan    = make(chan protocol.Event, 100)
	eventChan    = make(chan protocol.Event, 100)
	shutdownChan = make(chan struct{})
)

// DEVICE MATCHING ALGORITHM SPECIFICATION:
// =======================================
// 
// The device matching system uses a simplified embedding-based similarity approach
// with the following scoring components:
//
// 1. VOCABULARY-BASED SCORING:
//    - Each device name is converted to a feature vector based on configured vocabulary
//    - Each vocabulary term gets a score of 1.0 if found as substring in device name (case-insensitive)
//    - Character-based features add small bonuses: 0.1 per character mapped to vocabulary dimensions
//
// 2. SIMILARITY CALCULATION:
//    - Cosine similarity between target pattern and device name embeddings
//    - Range: 0.0 (no similarity) to 1.0 (identical)
//
// 3. SUBSTRING BONUS:
//    - Intelligent substring matching with scaled bonuses (0.3-0.7)
//    - Filters out generic words ("device", "audio", "virtual", "sound")
//    - Requires meaningful word matches (4+ characters)
//    - Bonus scales with number of matching meaningful words
//
// 4. MATCHING THRESHOLD:
//    - Minimum score of 0.9 required for device selection
//    - Prevents false matches on unrelated devices
//
// 5. ROBUSTNESS FEATURES:
//    - All text processing is case-insensitive
//    - Vocabulary terms are normalized (trimmed, lowercased)
//    - Handles special characters and spaces in device names
//    - Empty or invalid vocabulary gracefully handled
//
// Device name embeddings for smart matching using configurable vocabulary
func getNameEmbedding(name string, vocabulary []string) []float64 {
	// Normalize input name: lowercase and trim whitespace
	name = strings.ToLower(strings.TrimSpace(name))
	
	// Use default vocabulary if not configured
	if len(vocabulary) == 0 {
		vocabulary = getDefaultVocabulary()
	}
	
	// Ensure minimum embedding size to prevent issues
	if len(vocabulary) < 3 {
		slog.Warn("vocabulary too small, using default", "size", len(vocabulary))
		vocabulary = getDefaultVocabulary()
	}
	
	embedding := make([]float64, len(vocabulary))
	
	// Primary feature scoring: 1.0 for each vocabulary term found in name
	for i, term := range vocabulary {
		// Normalize vocabulary term: trim whitespace and convert to lowercase
		normalizedTerm := strings.ToLower(strings.TrimSpace(term))
		if normalizedTerm == "" {
			continue // Skip empty terms
		}
		
		if strings.Contains(name, normalizedTerm) {
			embedding[i] = 1.0
		}
	}
	
	// Character-based features: small bonuses for character distribution
	// This helps with fuzzy matching when vocabulary doesn't capture everything
	for _, char := range name {
		if char >= 'a' && char <= 'z' {
			// Map character to embedding dimension cyclically
			embedding[int(char-'a')%len(embedding)] += 0.1
		}
	}
	
	return embedding
}

// getDefaultVocabulary returns a sensible default vocabulary for audio device matching
func getDefaultVocabulary() []string {
	return []string{
		"steelseries", "sonar", "gaming", "chat", "aux", "media", "microphone",
		"realtek", "speakers", "bose", "headphones", "nc", "700", "audio",
		"playback", "output", "input", "usb", "wireless", "bluetooth",
	}
}

// Calculate cosine similarity between two embeddings
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}
	
	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	
	// Handle zero-length vectors
	if normA == 0 || normB == 0 {
		return 0.0
	}
	
	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// Find best matching device using embeddings and configurable vocabulary
func findBestDevice(targetName string, devices []AudioDevice, vocabulary []string) *AudioDevice {
	if len(devices) == 0 {
		return nil
	}
	
	// Normalize target name
	targetName = strings.TrimSpace(targetName)
	if targetName == "" {
		return nil
	}
	
	targetEmbedding := getNameEmbedding(targetName, vocabulary)
	bestScore := -1.0
	var bestDevice *AudioDevice
	
	for i := range devices {
		deviceEmbedding := getNameEmbedding(devices[i].Name, vocabulary)
		score := cosineSimilarity(targetEmbedding, deviceEmbedding)
		
		// Substring bonus: higher weight for direct substring matches
		// Check if any word from target appears in device name or vice versa
		targetLower := strings.ToLower(targetName)
		deviceLower := strings.ToLower(devices[i].Name)
		
		hasSubstringMatch := false
		substringScore := 0.0
		
		// Check if target is substring of device name
		if strings.Contains(deviceLower, targetLower) {
			hasSubstringMatch = true
			substringScore = 0.5
		}
		// Check if device name is substring of target  
		if strings.Contains(targetLower, deviceLower) {
			hasSubstringMatch = true
			substringScore = 0.5
		}
		
		// Check for individual word matches (more selective)
		targetWords := strings.Fields(targetLower)
		deviceWords := strings.Fields(deviceLower)
		
		// Common generic words that shouldn't contribute to matching
		genericWords := map[string]bool{
			"device": true, "audio": true, "virtual": true, "sound": true,
		}
		
		matchingWords := 0
		for _, targetWord := range targetWords {
			if len(targetWord) > 3 && !genericWords[targetWord] { // More stringent: 4+ chars, not generic
				for _, deviceWord := range deviceWords {
					// Remove common punctuation for better matching
					cleanDeviceWord := strings.Trim(deviceWord, "(),")
					if len(cleanDeviceWord) > 3 && !genericWords[cleanDeviceWord] {
						if strings.Contains(cleanDeviceWord, targetWord) || strings.Contains(targetWord, cleanDeviceWord) {
							matchingWords++
							hasSubstringMatch = true
							break
						}
					}
				}
			}
		}
		
		// Only give substantial bonus for meaningful matches
		if hasSubstringMatch && matchingWords > 0 {
			// Scale the bonus based on the number of matching words
			substringScore = 0.3 + (float64(matchingWords) * 0.2) // 0.3 base + 0.2 per word
			if substringScore > 0.7 {
				substringScore = 0.7 // Cap at 0.7
			}
		} else if hasSubstringMatch {
			substringScore = 0.3 // Lower bonus for whole-string matches without meaningful words
		}
		
		score += substringScore
		
		slog.Debug("device matching", 
			"target", targetName, 
			"device", devices[i].Name, 
			"base_score", score-substringScore, // Show score without substring bonus
			"substring_bonus", hasSubstringMatch,
			"final_score", score)
		
		if score > bestScore {
			bestScore = score
			bestDevice = &devices[i]
		}
	}
	
	const matchingThreshold = 0.9 // Requires high confidence for matching
	if bestScore > matchingThreshold {
		slog.Info("found matching device", 
			"target", targetName, 
			"matched", bestDevice.Name, 
			"score", bestScore)
		return bestDevice
	}
	
	slog.Debug("no device matched threshold", 
		"target", targetName, 
		"best_score", bestScore, 
		"threshold", matchingThreshold)
	return nil
}

// Enumerate audio playback devices
func enumerateAudioDevices() ([]AudioDevice, error) {
	oleLock.Lock()
	defer oleLock.Unlock()

	mmde, err := getDeviceEnumerator()
	if err != nil {
		return nil, fmt.Errorf("failed to get device enumerator: %w", err)
	}

	var collection *wca.IMMDeviceCollection
	err = mmde.EnumAudioEndpoints(wca.ERender, wca.DEVICE_STATE_ACTIVE, &collection)
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate audio endpoints: %w", err)
	}
	defer collection.Release()

	var count uint32
	err = collection.GetCount(&count)
	if err != nil {
		return nil, fmt.Errorf("failed to get device count: %w", err)
	}

	devices := make([]AudioDevice, 0, count)
	
	for i := uint32(0); i < count; i++ {
		var device *wca.IMMDevice
		err = collection.Item(i, &device)
		if err != nil {
			slog.Warn("failed to get device", "index", i, "err", err)
			continue
		}

		var deviceIDStr string
		err = device.GetId(&deviceIDStr)
		if err != nil {
			slog.Warn("failed to get device ID", "index", i, "err", err)
			device.Release()
			continue
		}
		idStr := deviceIDStr

		var propertyStore *wca.IPropertyStore
		err = device.OpenPropertyStore(wca.STGM_READ, &propertyStore)
		if err != nil {
			slog.Warn("failed to open property store", "index", i, "err", err)
			device.Release()
			continue
		}

		var propertyValue wca.PROPVARIANT
		err = propertyStore.GetValue(&wca.PKEY_Device_FriendlyName, &propertyValue)
		if err != nil {
			slog.Warn("failed to get device name", "index", i, "err", err)
			propertyStore.Release()
			device.Release()
			continue
		}

		name := propertyValue.String()
		
		devices = append(devices, AudioDevice{
			ID:   idStr,
			Name: name,
		})

		propertyStore.Release()
		device.Release()
	}

	slog.Info("enumerated audio devices", "count", len(devices))
	return devices, nil
}

// Get cached devices or refresh if needed
func getCachedDevices() []AudioDevice {
	deviceCacheLock.RLock()
	if time.Since(lastDeviceUpdate) < 30*time.Second && len(deviceCache) > 0 {
		devices := make([]AudioDevice, len(deviceCache))
		copy(devices, deviceCache)
		deviceCacheLock.RUnlock()
		return devices
	}
	deviceCacheLock.RUnlock()

	// Need to refresh cache
	devices, err := enumerateAudioDevices()
	if err != nil {
		slog.Error("failed to enumerate devices", "err", err)
		deviceCacheLock.RLock()
		devices = make([]AudioDevice, len(deviceCache))
		copy(devices, deviceCache)
		deviceCacheLock.RUnlock()
		return devices
	}

	deviceCacheLock.Lock()
	deviceCache = devices
	lastDeviceUpdate = time.Now()
	deviceCacheLock.Unlock()

	return devices
}

// Resolve device ID from name pattern
func resolveDeviceID(comboConfig *ComboConfig) string {
	// If deviceID is specified (backward compatibility), use it
	if comboConfig.DeviceID != "" {
		return comboConfig.DeviceID
	}

	devices := getCachedDevices()
	
	// Try primary device name
	if comboConfig.DeviceName != "" {
		if device := findBestDevice(comboConfig.DeviceName, devices, config.DeviceMatchingVocabulary); device != nil {
			return device.ID
		}
	}
	
	// Try fallback names
	for _, fallbackName := range comboConfig.FallbackNames {
		if device := findBestDevice(fallbackName, devices, config.DeviceMatchingVocabulary); device != nil {
			slog.Info("using fallback device", "combo", comboConfig.Combo, "target", comboConfig.DeviceName, "fallback", fallbackName, "matched", device.Name)
			return device.ID
		}
	}
	
	slog.Warn("no matching device found", "combo", comboConfig.Combo, "target", comboConfig.DeviceName)
	return ""
}

func loadConfig() {
	configLock.Lock()
	defer configLock.Unlock()

	data, err := os.ReadFile(configFile)
	if err != nil {
		slog.Warn("error reading config file", "err", err)
		return
	}

	var newConfig Config
	err = yaml.Unmarshal(data, &newConfig)
	if err != nil {
		slog.Warn("error parsing config file", "err", err)
		return
	}

	config = newConfig
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
	slog.Info("received event", "event", event.String())

	comboConfig := getComboConfig(event.Combo)
	if comboConfig == nil {
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

	deviceID := resolveDeviceID(comboConfig)
	if deviceID == "" {
		slog.Error("no device found for combo", "combo", event.Combo)
		return
	}

	err := setVolume(deviceID, volumeLevel)
	if err != nil {
		slog.Error("error setting volume", "deviceID", deviceID, "err", err)
	} else {
		slog.Info("set volume", "state", state, "deviceID", deviceID)
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
		slog.Info("sending set events to synchronize device state")
		configLock.RLock()
		combos := config.Combos
		configLock.RUnlock()

		for _, combo := range combos {
			// Resolve device ID for this combo
			deviceID := resolveDeviceID(&combo)
			if deviceID == "" {
				slog.Warn("no device found for combo, skipping", "combo", combo.Combo)
				continue
			}

			// Retrieve the current volume level
			currentVolume, err := getCurrentVolume(deviceID)
			if err != nil {
				slog.Error("error getting current volume", "deviceID", deviceID, "err", err)
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
			case <-shutdownChan:
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

	// Initialize COM for device enumeration
	err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	if err != nil {
		log.Fatalf("Failed to initialize COM: %v", err)
	}
	defer ole.CoUninitialize()

	// Initialize device cache
	slog.Info("initializing audio device cache")
	devices := getCachedDevices()
	slog.Info("discovered audio devices", "count", len(devices))
	for _, device := range devices {
		slog.Debug("found device", "name", device.Name, "id", device.ID)
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

	slog.Info("application is running. press Ctrl+C to exit")
	<-sigs
	slog.Info("interrupt signal received. initiating shutdown")

	// Signal all goroutines to stop
	close(shutdownChan)

	// Allow some time for goroutines to finish
	time.Sleep(1 * time.Second)
	slog.Info("application terminated gracefully")
}
