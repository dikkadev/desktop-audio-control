# Desktop Audio Control

This project is a custom desktop audio control system using various hardware components. The parts used in this project include:

- Waveshare RP2040
- 5x soldered I2C enabled rotary encoders ([Rotary Encoder Board with EasyC](https://soldered.com/product/rotary-encoder-board-with-easyc/))
- 5x 1.3in OLED screens from AZDelivery ([OLED Screen](https://www.amazon.de/dp/B07V9SLQ6W))
- TCA9548A based I2C multiplexer ([I2C Multiplexer TCA9548A Breakout](https://soldered.com/product/i2c-multiplexer-tca9548a-breakout/))

## Features

- Control audio levels for multiple devices using rotary encoders
- Display current audio levels on OLED screens
- **Smart Device Matching**: Intelligent audio device detection using semantic similarity
- **Configurable Vocabulary**: Customize keywords for device matching to suit your setup
- **Fallback Device Support**: Automatic fallback to alternative devices when primary devices are unavailable
- Customizable configuration for different audio devices
- Automatic synchronization of device states
- 3D printable parts to hold all the electronics in a nice fashion available at [OnShape](https://cad.onshape.com/documents/7b12144c499674f4b30f7002/w/6ec67aca20472cc54069a1b1/e/79128cd30b664a04af6ef011?renderMode=0&uiState=675ed89cb702172b114da0a2)

## Smart Device Matching System

This application features an advanced device matching system that uses semantic similarity to automatically detect and connect to your audio devices, even when device IDs change due to driver updates or hardware changes.

### How It Works

The system uses a **vocabulary-based embedding approach** with the following components:

1. **Vocabulary-Based Scoring**: Each device name is converted to a feature vector based on a configurable vocabulary
2. **Cosine Similarity**: Calculates similarity between target device patterns and actual device names
3. **Substring Bonus**: Provides additional matching confidence for direct substring matches
4. **Threshold-Based Selection**: Only matches devices above a confidence threshold to prevent false matches

### Device Matching Algorithm

The matching algorithm works as follows:

1. **Vocabulary Mapping**: Each vocabulary term gets a score of 1.0 if found in the device name (case-insensitive)
2. **Character Features**: Small bonuses (0.1) are added based on character distribution for fuzzy matching
3. **Similarity Calculation**: Cosine similarity is calculated between target and device embeddings (range: 0.0-1.0)
4. **Substring Bonus**: Intelligent substring matching with scaled bonuses (0.3-0.7), filtering generic words
5. **Threshold Check**: Devices must score above 0.9 to be considered a match

### Configuration

#### Basic Device Configuration

```yaml
combos:
  - combo: 0
    deviceName: "SteelSeries Sonar Gaming"
  - combo: 1
    deviceName: "Bose NC 700 Headphones"
    fallbackNames: ["Realtek Speakers", "Speakers Realtek"]
```

#### Configurable Vocabulary

Customize the vocabulary used for device matching by adding a `deviceMatchingVocabulary` section to your `config.yaml`:

```yaml
deviceMatchingVocabulary:
  # Brand names
  - "steelseries"
  - "realtek" 
  - "bose"
  - "logitech"
  - "corsair"
  
  # Product series/lines
  - "sonar"
  - "arctis"
  - "nc"
  - "700"
  
  # Device types
  - "gaming"
  - "chat"
  - "media"
  - "speakers"
  - "headphones"
  - "microphone"
  
  # Audio descriptors
  - "audio"
  - "output"
  - "input"
  - "surround"
  
  # Connection types
  - "usb"
  - "wireless"
  - "bluetooth"
```

**Important Notes:**
- **Order matters**: Each vocabulary term becomes a dimension in the feature vector
- **Case insensitive**: All matching is done case-insensitively
- **Partial matching**: Terms match if found anywhere in the device name
- **Normalization**: Vocabulary terms are automatically trimmed and normalized
- **Default fallback**: If no vocabulary is specified, a sensible default is used

#### Example Vocabulary Sets

**For Gaming Setups:**
```yaml
deviceMatchingVocabulary:
  - "steelseries"
  - "corsair"
  - "razer"
  - "gaming"
  - "headset"
  - "microphone"
  - "surround"
  - "7.1"
  - "usb"
```

**For Professional Audio:**
```yaml
deviceMatchingVocabulary:
  - "audio"
  - "interface"
  - "studio"
  - "monitor"
  - "professional"
  - "xlr"
  - "phantom"
  - "condenser"
  - "dynamic"
```

**For Home Theater:**
```yaml
deviceMatchingVocabulary:
  - "speakers"
  - "subwoofer"
  - "surround"
  - "5.1"
  - "7.1"
  - "atmos"
  - "hdmi"
  - "optical"
  - "receiver"
```

### Troubleshooting Device Matching

1. **Enable Debug Logging**: Set log level to debug to see matching scores
2. **Check Vocabulary**: Ensure your vocabulary includes terms from your device names
3. **Test Matching**: Use substring matching as a fallback if semantic matching fails
4. **Adjust Threshold**: The default threshold is 0.9, but this may need adjustment for your setup

### Robustness Features

- **Case Insensitive**: All text processing handles mixed case automatically
- **Whitespace Handling**: Leading/trailing spaces are automatically trimmed
- **Special Characters**: Handles special characters and Unicode in device names
- **Empty Vocabulary Protection**: Automatically falls back to default vocabulary if configuration is invalid
- **Driver Update Resilience**: Device matching works even when Windows driver updates change device IDs

## Installation and Setup

1. Clone this repository
2. Ensure Go is installed on your system
3. Install dependencies: `go mod tidy`
4. Configure your devices in `config.yaml`
5. Build the application: `go build`
6. Run the application

## Configuration Reference

See `config.yaml` for a complete configuration example with detailed comments explaining each option.
