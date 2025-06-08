package main

import (
	"math/rand"
	"strings"
	"testing"
	"time"
)

// FuzzyTestConfig controls the fuzzy testing parameters
type FuzzyTestConfig struct {
	NumIterations      int
	MaxStringLength    int
	IncludeUnicode     bool
	IncludeSpaces      bool
	IncludePunctuation bool
}

// generateRandomString creates a random string for fuzzy testing
func generateRandomString(config FuzzyTestConfig) string {
	if config.MaxStringLength == 0 {
		config.MaxStringLength = 50
	}

	length := rand.Intn(config.MaxStringLength) + 1
	var chars []rune

	// Basic ASCII letters and numbers
	for i := 'a'; i <= 'z'; i++ {
		chars = append(chars, i)
	}
	for i := 'A'; i <= 'Z'; i++ {
		chars = append(chars, i)
	}
	for i := '0'; i <= '9'; i++ {
		chars = append(chars, i)
	}

	// Add spaces if enabled
	if config.IncludeSpaces {
		chars = append(chars, ' ', '\t', '\n')
	}

	// Add punctuation if enabled
	if config.IncludePunctuation {
		punctuation := []rune{'!', '@', '#', '$', '%', '^', '&', '*', '(', ')', '-', '_', '+', '=',
			'[', ']', '{', '}', '|', '\\', ':', ';', '"', '\'', '<', '>', ',', '.', '?', '/'}
		chars = append(chars, punctuation...)
	}

	// Add Unicode characters if enabled
	if config.IncludeUnicode {
		// Add some common Unicode characters
		unicode_chars := []rune{'ñ', 'ü', 'ö', 'ä', 'é', 'è', 'ç', '中', '文', '日', '本', '語'}
		chars = append(chars, unicode_chars...)
	}

	result := make([]rune, length)
	for i := 0; i < length; i++ {
		result[i] = chars[rand.Intn(len(chars))]
	}

	return string(result)
}

// generateMalformedVocabulary creates vocabulary with various edge cases
func generateMalformedVocabulary() []string {
	return []string{
		"",                    // Empty string
		"   ",                 // Only spaces
		"a",                   // Single character
		"ab",                  // Two characters
		"UPPERCASE",           // All uppercase
		"MiXeD cAsE",          // Mixed case
		"with spaces",         // Contains spaces
		"with\ttabs",          // Contains tabs
		"with\nnewlines",      // Contains newlines
		"special!@#$%chars",   // Special characters
		"unicode文字",          // Unicode characters
		"very_long_vocabulary_term_that_exceeds_normal_length_expectations_and_might_cause_issues",
		"123numbers456",       // Contains numbers
		"   leading_spaces",   // Leading spaces
		"trailing_spaces   ",  // Trailing spaces
		"multiple   spaces",   // Multiple spaces
		"(parentheses)",       // Parentheses
		"[brackets]",          // Brackets
		"{braces}",            // Braces
		"under_scores",        // Underscores
		"hyphen-ated",         // Hyphens
		"dots.in.name",        // Dots
		"slash/separated",     // Slashes
		"back\\slash",         // Backslashes
	}
}

// TestFuzzyDeviceMatching performs fuzzy testing on the device matching system
func TestFuzzyDeviceMatching(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping fuzzy test in short mode")
	}

	rand.Seed(time.Now().UnixNano())

	// Test devices with various characteristics
	testDevices := []AudioDevice{
		{ID: "1", Name: "Normal Device Name"},
		{ID: "2", Name: "Device with (Parentheses) and [Brackets]"},
		{ID: "3", Name: "UPPERCASE DEVICE NAME"},
		{ID: "4", Name: "device with unicode 文字 characters"},
		{ID: "5", Name: "Device\twith\ttabs\tand\nnewlines"},
		{ID: "6", Name: "   Device with leading and trailing spaces   "},
		{ID: "7", Name: "Device-with-hyphens_and_underscores"},
		{ID: "8", Name: "Device.with.dots.and/slashes\\backslashes"},
		{ID: "9", Name: "Device with multiple    spaces"},
		{ID: "10", Name: "Special!@#$%^&*()Characters"},
	}

	config := FuzzyTestConfig{
		NumIterations:      100,
		MaxStringLength:    30,
		IncludeUnicode:     true,
		IncludeSpaces:      true,
		IncludePunctuation: true,
	}

	defaultVocab := getDefaultVocabulary()
	malformedVocab := generateMalformedVocabulary()

	t.Logf("Running %d fuzzy test iterations", config.NumIterations)

	panics := 0
	errors := 0

	for i := 0; i < config.NumIterations; i++ {
		// Test with random target names
		randomTarget := generateRandomString(config)

		// Test with different vocabularies
		vocabularies := [][]string{
			defaultVocab,
			malformedVocab,
			{}, // Empty vocabulary
			{"single"}, // Single term vocabulary
			generateMalformedVocabulary()[:5], // Subset of malformed vocab
		}

		for _, vocab := range vocabularies {
			func() {
				defer func() {
					if r := recover(); r != nil {
						panics++
						t.Logf("PANIC with target='%s', vocab=%v: %v", randomTarget, vocab, r)
					}
				}()

				// This should never panic, regardless of input
				result := findBestDevice(randomTarget, testDevices, vocab)

				// Basic sanity checks
				if result != nil {
					if result.ID == "" {
						errors++
						t.Logf("ERROR: Found device with empty ID for target='%s'", randomTarget)
					}
					if result.Name == "" {
						errors++
						t.Logf("ERROR: Found device with empty Name for target='%s'", randomTarget)
					}
				}
			}()
		}
	}

	t.Logf("Fuzzy testing completed: %d panics, %d errors out of %d iterations",
		panics, errors, config.NumIterations*5) // 5 vocabularies tested

	if panics > 0 {
		t.Errorf("Fuzzy testing found %d panics - the system is not robust enough", panics)
	}

	if errors > 0 {
		t.Errorf("Fuzzy testing found %d errors - the system has logic issues", errors)
	}
}

// TestFuzzyEmbeddingGeneration tests the embedding generation with malformed inputs
func TestFuzzyEmbeddingGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping fuzzy test in short mode")
	}

	rand.Seed(time.Now().UnixNano())

	config := FuzzyTestConfig{
		NumIterations:      50,
		MaxStringLength:    100,
		IncludeUnicode:     true,
		IncludeSpaces:      true,
		IncludePunctuation: true,
	}

	malformedVocab := generateMalformedVocabulary()
	panics := 0

	for i := 0; i < config.NumIterations; i++ {
		randomName := generateRandomString(config)

		func() {
			defer func() {
				if r := recover(); r != nil {
					panics++
					t.Logf("PANIC in getNameEmbedding with name='%s', vocab=%v: %v",
						randomName, malformedVocab, r)
				}
			}()

			// This should never panic
			embedding := getNameEmbedding(randomName, malformedVocab)

			// Basic sanity checks
			if len(embedding) == 0 {
				t.Logf("WARNING: Empty embedding for name='%s'", randomName)
			}

			// Check for NaN or infinite values
			for j, val := range embedding {
				if val != val { // NaN check
					t.Errorf("NaN value at index %d for name='%s'", j, randomName)
				}
				if val > 1e10 || val < -1e10 { // Check for very large values (infinity-like)
					t.Errorf("Infinite value at index %d for name='%s'", j, randomName)
				}
			}
		}()
	}

	if panics > 0 {
		t.Errorf("Embedding generation found %d panics - the system is not robust enough", panics)
	}
}

// TestEdgeCaseVocabulary tests vocabulary edge cases
func TestEdgeCaseVocabulary(t *testing.T) {
	edgeCases := []struct {
		name  string
		vocab []string
	}{
		{"nil vocabulary", nil},
		{"empty vocabulary", []string{}},
		{"single empty string", []string{""}},
		{"only spaces", []string{"   ", "\t", "\n"}},
		{"very long terms", []string{strings.Repeat("a", 1000)}},
		{"unicode terms", []string{"文字", "🎵", "café"}},
		{"mixed valid/invalid", []string{"valid", "", "   ", "also_valid"}},
	}

	testName := "Test Device"

	for _, tc := range edgeCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("PANIC with %s vocabulary: %v", tc.name, r)
				}
			}()

			// Should not panic with any vocabulary
			embedding := getNameEmbedding(testName, tc.vocab)

			// Should always return a valid embedding
			if len(embedding) == 0 {
				t.Errorf("Empty embedding with %s vocabulary", tc.name)
			}
		})
	}
}

// TestUnicodeRobustness specifically tests Unicode handling
func TestUnicodeRobustness(t *testing.T) {
	unicodeTestCases := []struct {
		name   string
		target string
		device string
	}{
		{"Chinese characters", "音频设备", "音频设备 (Audio Device)"},
		{"Japanese characters", "オーディオ", "オーディオデバイス"},
		{"Arabic characters", "صوت", "جهاز صوت"},
		{"Emoji", "🎵 Audio", "🎵 Music Device 🎶"},
		{"Mixed scripts", "Audio音频", "Audio音频Device"},
		{"Accented characters", "Café Audio", "Café Audio Device"},
		{"Cyrillic", "Аудио", "Аудио устройство"},
	}

	vocab := append(getDefaultVocabulary(), "音频", "オーディオ", "صوت", "аудио")

	for _, tc := range unicodeTestCases {
		t.Run(tc.name, func(t *testing.T) {
			devices := []AudioDevice{{ID: "1", Name: tc.device}}

			defer func() {
				if r := recover(); r != nil {
					t.Errorf("PANIC with Unicode test %s: %v", tc.name, r)
				}
			}()

			result := findBestDevice(tc.target, devices, vocab)
			// We don't require a match, just that it doesn't crash
			t.Logf("Unicode test %s: target='%s', device='%s', matched=%t",
				tc.name, tc.target, tc.device, result != nil)
		})
	}
} 