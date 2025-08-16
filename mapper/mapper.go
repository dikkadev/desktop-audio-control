package mapper

import (
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
)

type Mapper interface {
	Map(x float64) float64
	MapReverse(x float64) float64
	fmt.Stringer
}

type CustomMapper struct{}

func (CustomMapper) String() string {
	return "CustomMapper"
}

// Shared shape/knee parameters for custom mapper (used by forward + reverse)
const (
	customKneeX = 0.7 // switch to linear above this input
	customK     = 1.9
	customL     = 3.95
	customM     = 0.6
)

// Nonlinear forward on [0, customKneeX). Kept private and math-only.
// y = x^k / (x^l + (1-x)^m)
func customForwardNonlinear(x float64) float64 {
	numerator := math.Pow(x, customK)
	denominator := math.Pow(x, customL) + math.Pow(1-x, customM)
	return numerator / denominator
}

func (CustomMapper) Map(x float64) float64 {
	if x < 0 {
		x = 0
	} else if x > 1 {
		x = 1
	}

	if x < customKneeX {
		return customForwardNonlinear(x)
	}
	return x
}

func (CustomMapper) MapReverse(targetY float64) float64 {
	if targetY < 0 {
		targetY = 0
	} else if targetY > 1 {
		targetY = 1
	}

	if targetY >= customKneeX {
		return targetY
	}

	// Strictly increasing on [0, customKneeX] ⇒ unique preimage.
	// We solve customForwardNonlinear(x) = targetY on [0, customKneeX]
	lowerBound := 0.0
	upperBound := customKneeX

	// Initial guess: near the knee f(x)≈x; elsewhere, still a decent seed.
	xGuess := targetY
	if xGuess <= 0 {
		xGuess = 1e-9
	} else if xGuess >= customKneeX {
		xGuess = customKneeX - 1e-9
	}

	// Hybrid Newton–bisection.
	for iter := 0; iter < 24; iter++ {
		yAtGuess := customForwardNonlinear(xGuess)

		// Derivative of y wrt x (reverse-only; keep local):
		// Let A = x^k, B = x^l + (1-x)^m, y = A/B
		// A' = k*x^(k-1)
		// B' = l*x^(l-1) - m*(1-x)^(m-1)
		// y' = (A'*B - A*B') / B^2
		A := math.Pow(xGuess, customK)
		B := math.Pow(xGuess, customL) + math.Pow(1-xGuess, customM)
		dA := customK * math.Pow(xGuess, customK-1)
		dB := customL*math.Pow(xGuess, customL-1) - customM*math.Pow(1-xGuess, customM-1)
		slopeAtGuess := (dA*B - A*dB) / (B * B)

		// Try a Newton step when the slope is well-behaved; otherwise bisect.
		newtonGuess := xGuess
		useNewton := slopeAtGuess > 1e-12 && slopeAtGuess < 1e12 && !math.IsNaN(slopeAtGuess) && !math.IsInf(slopeAtGuess, 0)
		if useNewton {
			newtonGuess = xGuess - (yAtGuess-targetY)/slopeAtGuess
		}

		if useNewton && newtonGuess > lowerBound && newtonGuess < upperBound && !math.IsNaN(newtonGuess) && !math.IsInf(newtonGuess, 0) {
			xGuess = newtonGuess
		} else {
			xGuess = 0.5 * (lowerBound + upperBound) // bisection fallback
		}

		// Tighten bracket according to sign of residual
		if customForwardNonlinear(xGuess) > targetY {
			upperBound = xGuess
		} else {
			lowerBound = xGuess
		}

		// Converged when bracket is sufficiently tight
		if upperBound-lowerBound < 1e-9 {
			break
		}
	}

	// Final clamp to guard against tiny numerical drift
	if xGuess < 0 {
		return 0
	}
	if xGuess > customKneeX {
		return customKneeX
	}
	return xGuess
}

type LinearMapper struct{}

func (LinearMapper) String() string {
	return "LinearMapper"
}

func (LinearMapper) Map(x float64) float64 {
	return x
}

func (LinearMapper) MapReverse(x float64) float64 {
	return x
}

type ExpMapper struct {
	exp int
}

func (em ExpMapper) String() string {
	return fmt.Sprintf("ExpMapper^%d", em.exp)
}

func (em ExpMapper) Map(x float64) float64 {
	return math.Pow(x, float64(em.exp))
}

func (em ExpMapper) MapReverse(x float64) float64 {
	if em.exp <= 0 {
		return 0
	}
	if x < 0 {
		x = 0
	} else if x > 1 {
		x = 1
	}
	return math.Pow(x, 1.0/float64(em.exp))
}

var CurrentMapper Mapper = LinearMapper{}

func SetCurrentMapper(config string) {
	config = strings.ToLower(config)
	switch {
	case config == "custom":
		CurrentMapper = CustomMapper{}
	case config == "linear":
		CurrentMapper = LinearMapper{}
	case strings.HasPrefix(config, "exp"):
		expStr := strings.TrimPrefix(config, "exp")
		if exp, err := strconv.Atoi(expStr); err == nil {
			CurrentMapper = ExpMapper{exp: exp}
		} else {
			slog.Error("error on setting volume mapper based on config", "e", err)
			CurrentMapper = LinearMapper{}
		}
	}

	slog.Debug("set current volume mapper", "mapper", CurrentMapper)
}
