package main

import (
	"math"
	"strings"
	"testing"
)

func TestGetNameEmbedding(t *testing.T) {
	// Use default vocabulary for consistency
	defaultVocabulary := getDefaultVocabulary()
	
	tests := []struct {
		name     string
		input    string
		expected []string // features that should be present (1.0)
	}{
		{
			name:     "SteelSeries Gaming",
			input:    "SteelSeries Sonar - Gaming (SteelSeries Sonar Virtual Audio Device)",
			expected: []string{"steelseries", "sonar", "gaming"},
		},
		{
			name:     "Realtek Speakers",
			input:    "Speakers (Realtek(R) Audio)",
			expected: []string{"realtek", "speakers"},
		},
		{
			name:     "Bose Headphones",
			input:    "Headphones (Bose NC 700 Headphones)",
			expected: []string{"bose", "headphones", "nc", "700"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			embedding := getNameEmbedding(tt.input, defaultVocabulary)
			
			// Check that expected features are present
			for _, expectedFeature := range tt.expected {
				found := false
				for i, feature := range defaultVocabulary {
					if feature == expectedFeature && embedding[i] >= 1.0 {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected feature '%s' not found in embedding for input '%s'", expectedFeature, tt.input)
				}
			}
		})
	}
}

func TestGetNameEmbeddingWithCustomVocabulary(t *testing.T) {
	customVocabulary := []string{"custom", "test", "device", "audio"}
	
	tests := []struct {
		name     string
		input    string
		vocab    []string
		expected map[string]bool // features that should be present
	}{
		{
			name:     "custom vocabulary match",
			input:    "Custom Test Device",
			vocab:    customVocabulary,
			expected: map[string]bool{"custom": true, "test": true, "device": true},
		},
		{
			name:     "empty vocabulary uses default",
			input:    "SteelSeries Gaming",
			vocab:    []string{},
			expected: map[string]bool{"steelseries": true, "gaming": true},
		},
		{
			name:     "vocabulary with spaces and mixed case",
			input:    "My Audio Device",
			vocab:    []string{"  My  ", "AUDIO", "device "},
			expected: map[string]bool{"my": true, "audio": true, "device": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			embedding := getNameEmbedding(tt.input, tt.vocab)
			
			// For empty vocab test, use default vocabulary for checking
			checkVocab := tt.vocab
			if len(tt.vocab) == 0 {
				checkVocab = getDefaultVocabulary()
			}
			
			for expectedFeature := range tt.expected {
				found := false
				for i, feature := range checkVocab {
					normalizedFeature := strings.ToLower(strings.TrimSpace(feature))
					if normalizedFeature == expectedFeature && embedding[i] >= 1.0 {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected feature '%s' not found in embedding for input '%s' with vocab %v", expectedFeature, tt.input, tt.vocab)
				}
			}
		})
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []float64
		b        []float64
		expected float64
		delta    float64
	}{
		{
			name:     "identical vectors",
			a:        []float64{1, 0, 1, 0},
			b:        []float64{1, 0, 1, 0},
			expected: 1.0,
			delta:    0.001,
		},
		{
			name:     "orthogonal vectors",
			a:        []float64{1, 0, 0, 0},
			b:        []float64{0, 1, 0, 0},
			expected: 0.0,
			delta:    0.001,
		},
		{
			name:     "zero vectors",
			a:        []float64{0, 0, 0, 0},
			b:        []float64{0, 0, 0, 0},
			expected: 0.0,
			delta:    0.001,
		},
		{
			name:     "empty vectors",
			a:        []float64{},
			b:        []float64{},
			expected: 0.0,
			delta:    0.001,
		},
		{
			name:     "mismatched length vectors",
			a:        []float64{1, 0},
			b:        []float64{1, 0, 1},
			expected: 0.0,
			delta:    0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cosineSimilarity(tt.a, tt.b)
			if math.Abs(result-tt.expected) > tt.delta {
				t.Errorf("Expected similarity %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestFindBestDevice(t *testing.T) {
	devices := []AudioDevice{
		{ID: "1", Name: "SteelSeries Sonar - Gaming (SteelSeries Sonar Virtual Audio Device)"},
		{ID: "2", Name: "SteelSeries Sonar - Chat (SteelSeries Sonar Virtual Audio Device)"},
		{ID: "3", Name: "SteelSeries Sonar - Media (SteelSeries Sonar Virtual Audio Device)"},
		{ID: "4", Name: "Speakers (Realtek(R) Audio)"},
		{ID: "5", Name: "Headphones (Bose NC 700 Headphones)"},
		{ID: "6", Name: "Random Audio Device"},
	}

	defaultVocabulary := getDefaultVocabulary()
	t.Logf("Default vocabulary: %v", defaultVocabulary)

	tests := []struct {
		name       string
		target     string
		vocabulary []string
		expectedID string
		shouldFind bool
	}{
		{
			name:       "exact gaming match",
			target:     "SteelSeries Sonar Gaming",
			vocabulary: defaultVocabulary,
			expectedID: "1",
			shouldFind: true,
		},
		{
			name:       "chat match",
			target:     "SteelSeries Sonar Chat",
			vocabulary: defaultVocabulary,
			expectedID: "2",
			shouldFind: true,
		},
		{
			name:       "realtek speakers",
			target:     "Realtek Speakers",
			vocabulary: defaultVocabulary,
			expectedID: "4",
			shouldFind: true,
		},
		{
			name:       "bose headphones",
			target:     "Bose NC 700 Headphones",
			vocabulary: defaultVocabulary,
			expectedID: "5",
			shouldFind: true,
		},
		{
			name:       "no match",
			target:     "NonExistent Device",
			vocabulary: defaultVocabulary,
			expectedID: "",
			shouldFind: false,
		},
		{
			name:       "empty target",
			target:     "",
			vocabulary: defaultVocabulary,
			expectedID: "",
			shouldFind: false,
		},
		{
			name:       "whitespace target",
			target:     "   ",
			vocabulary: defaultVocabulary,
			expectedID: "",
			shouldFind: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For debugging the realtek test
			if tt.name == "realtek speakers" {
				t.Logf("Testing target: '%s'", tt.target)
				for i, device := range devices {
					t.Logf("Device %d: '%s'", i, device.Name)
					targetEmbedding := getNameEmbedding(tt.target, tt.vocabulary)
					deviceEmbedding := getNameEmbedding(device.Name, tt.vocabulary)
					score := cosineSimilarity(targetEmbedding, deviceEmbedding)
					
					// Use the same substring matching logic as the main function
					targetLower := strings.ToLower(tt.target)
					deviceLower := strings.ToLower(device.Name)
					
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
					
					t.Logf("  Score: %f (substring bonus: %t, bonus: %f)", score, hasSubstringMatch, substringScore)
				}
			}
			
			result := findBestDevice(tt.target, devices, tt.vocabulary)
			
			if tt.shouldFind {
				if result == nil {
					t.Errorf("Expected to find device for target '%s', but got nil", tt.target)
				} else if result.ID != tt.expectedID {
					t.Errorf("Expected device ID '%s', got '%s' for target '%s'", tt.expectedID, result.ID, tt.target)
				}
			} else {
				if result != nil {
					t.Errorf("Expected no match for target '%s', but got device '%s'", tt.target, result.Name)
				}
			}
		})
	}
}

func TestFindBestDeviceWithFallback(t *testing.T) {
	devices := []AudioDevice{
		{ID: "1", Name: "Speakers (Realtek(R) Audio)"},
		{ID: "2", Name: "Some Other Device"},
	}

	defaultVocabulary := getDefaultVocabulary()

	// Test that when Bose headphones aren't available, it falls back to Realtek
	target := "Bose NC 700 Headphones"
	result := findBestDevice(target, devices, defaultVocabulary)
	
	// Should not find Bose since it's not in the device list
	if result != nil {
		t.Errorf("Expected no match for Bose when not available, but got '%s'", result.Name)
	}

	// Now test fallback to Realtek
	fallbackTarget := "Realtek Speakers"
	fallbackResult := findBestDevice(fallbackTarget, devices, defaultVocabulary)
	
	if fallbackResult == nil {
		t.Error("Expected to find Realtek speakers as fallback")
	} else if fallbackResult.ID != "1" {
		t.Errorf("Expected Realtek speakers (ID: 1), got ID: %s", fallbackResult.ID)
	}
}

func TestFindBestDeviceWithCustomVocabulary(t *testing.T) {
	devices := []AudioDevice{
		{ID: "1", Name: "Custom Gaming Device"},
		{ID: "2", Name: "Standard Audio Output"},
		{ID: "3", Name: "Professional Microphone"},
	}

	customVocabulary := []string{"custom", "gaming", "professional", "microphone", "standard"}

	tests := []struct {
		name       string
		target     string
		expectedID string
		shouldFind bool
	}{
		{
			name:       "custom gaming match",
			target:     "Custom Gaming",
			expectedID: "1",
			shouldFind: true,
		},
		{
			name:       "professional microphone match",
			target:     "Professional Microphone",
			expectedID: "3",
			shouldFind: true,
		},
		{
			name:       "standard audio match",
			target:     "Standard Audio",
			expectedID: "2",
			shouldFind: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findBestDevice(tt.target, devices, customVocabulary)
			
			if tt.shouldFind {
				if result == nil {
					t.Errorf("Expected to find device for target '%s', but got nil", tt.target)
				} else if result.ID != tt.expectedID {
					t.Errorf("Expected device ID '%s', got '%s' for target '%s'", tt.expectedID, result.ID, tt.target)
				}
			} else {
				if result != nil {
					t.Errorf("Expected no match for target '%s', but got device '%s'", tt.target, result.Name)
				}
			}
		})
	}
}

func TestGetDefaultVocabulary(t *testing.T) {
	vocab := getDefaultVocabulary()
	
	// Check that we have a reasonable number of terms
	if len(vocab) < 10 {
		t.Errorf("Expected at least 10 vocabulary terms, got %d", len(vocab))
	}
	
	// Check that essential terms are present
	expectedTerms := []string{"steelseries", "sonar", "gaming", "realtek", "speakers", "bose", "headphones"}
	for _, term := range expectedTerms {
		found := false
		for _, vocabTerm := range vocab {
			if vocabTerm == term {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected term '%s' not found in default vocabulary", term)
		}
	}
} 