package mapper

import (
	"fmt"
	"math"
	"testing"
)

// ---------- helpers ----------

func almostEqual(a, b, eps float64) bool {
	if math.IsNaN(a) || math.IsNaN(b) {
		return false
	}
	diff := math.Abs(a - b)
	if a == b {
		return true
	}
	// ULP-aware-ish: absolute + relative tolerance
	return diff <= eps || diff <= eps*math.Max(math.Abs(a), math.Abs(b))
}

func in01(x float64) bool { return x >= 0 && x <= 1 }

// Compute the highest nonlinear output just below the knee.
// (There is a small unreachable y-gap: (yMaxNonlinear, customKneeX))
func yMaxNonlinearAtKnee() float64 {
	const tiny = 1e-9
	return customForwardNonlinear(customKneeX - tiny)
}

// Grid generator
func linspace(n int) []float64 {
	out := make([]float64, n)
	if n == 1 {
		out[0] = 0
		return out
	}
	step := 1.0 / float64(n-1)
	for i := 0; i < n; i++ {
		out[i] = float64(i) * step
	}
	return out
}

// ---------- Linear mapper tests ----------

func TestLinearMapper_RoundTripAndBasics(t *testing.T) {
	m := LinearMapper{}
	xs := linspace(1001)
	const eps = 1e-12

	// round-trip both ways
	for _, x := range xs {
		y := m.Map(x)
		if !almostEqual(y, x, eps) {
			t.Fatalf("linear Map(x) != x at x=%.9f: got %.9f", x, y)
		}
		x2 := m.MapReverse(y)
		if !almostEqual(x2, x, eps) {
			t.Fatalf("linear MapReverse(Map(x)) != x at x=%.9f: got %.9f", x, x2)
		}
	}

	// monotonic & range
	prev := -1.0
	for _, x := range xs {
		y := m.Map(x)
		if y < prev-eps {
			t.Fatalf("linear not monotone: y(x=%.6f)=%.9f < prev %.9f", x, y, prev)
		}
		prev = y
		if !in01(y) {
			t.Fatalf("linear out of range at x=%.6f: %.9f", x, y)
		}
	}
}

// ---------- Exp mapper tests ----------

func TestExpMapper_RoundTripVariousExp(t *testing.T) {
	exps := []int{1, 2, 3, 4, 8}
	xs := linspace(1001)
	const eps = 2e-12 // pure powers → very tight

	for _, e := range exps {
		m := ExpMapper{exp: e}
		t.Run(fmt.Sprintf("exp=%d", e), func(t *testing.T) {
			// round-trip both ways
			for _, x := range xs {
				y := m.Map(x)
				if !in01(y) {
					t.Fatalf("exp^%d Map out of range at x=%.6f: %.9f", e, x, y)
				}
				x2 := m.MapReverse(y)
				if !almostEqual(x2, x, 1e-9) {
					t.Fatalf("exp^%d MapReverse(Map(x)) err: x=%.9f got %.9f", e, x, x2)
				}
			}

			// endpoints
			if m.Map(0) != 0 || m.MapReverse(0) != 0 {
				t.Fatalf("exp^%d endpoints at 0 broken", e)
			}
			if m.Map(1) != 1 || m.MapReverse(1) != 1 {
				t.Fatalf("exp^%d endpoints at 1 broken", e)
			}
		})
	}
}

func TestExpMapper_ExpLE0(t *testing.T) {
	m := ExpMapper{exp: 0}
	if want, got := 0.0, m.MapReverse(0.3); got != want {
		t.Fatalf("exp<=0 MapReverse should return 0, got %.6f", got)
	}
}

// ---------- Custom mapper tests ----------

func TestCustomMapper_RoundTrip_ForwardThenReverse(t *testing.T) {
	m := CustomMapper{}
	xs := linspace(2001)
	const eps = 1e-7 // numeric inversion tolerance

	for _, x := range xs {
		y := m.Map(x)

		// range guarantee
		if !in01(y) || math.IsNaN(y) || math.IsInf(y, 0) {
			t.Fatalf("custom Map out of range/NaN at x=%.9f: %.9f", x, y)
		}

		x2 := m.MapReverse(y)

		// For x < knee → should reconstruct x (within eps)
		// For x >= knee → identity branch both ways
		if x < customKneeX {
			if !almostEqual(x2, x, eps) {
				t.Fatalf("custom round-trip (fwd->rev) failed for x=%.9f: got %.9f (y=%.9f)", x, x2, y)
			}
		} else {
			if !almostEqual(x2, x, eps) {
				t.Fatalf("custom round-trip (linear) failed for x=%.9f: got %.9f", x, x2)
			}
		}
	}
}

func TestCustomMapper_RoundTrip_ReverseThenForward_WithKneeGapHandling(t *testing.T) {
	m := CustomMapper{}
	ys := linspace(2001)
	const eps = 1e-7

	yMaxNonlin := yMaxNonlinearAtKnee() // ≈ f(0.7-)
	// sanity: yMaxNonlin < customKneeX
	if !(yMaxNonlin < customKneeX) {
		t.Fatalf("expected yMaxNonlin < knee; got yMaxNonlin=%.9f knee=%.9f", yMaxNonlin, customKneeX)
	}

	for _, y := range ys {
		x := m.MapReverse(y)
		y2 := m.Map(x)

		if y < yMaxNonlin-1e-9 {
			// In the nonlinear reachable range → should reconstruct y
			if !almostEqual(y2, y, eps) {
				t.Fatalf("custom (rev->fwd) failed in reachable range: y=%.9f got y2=%.9f x=%.9f", y, y2, x)
			}
		} else if y >= customKneeX {
			// Linear branch → identity both ways
			if !almostEqual(y2, y, eps) || !almostEqual(x, y, eps) {
				t.Fatalf("custom (rev->fwd) failed in linear range: y=%.9f got y2=%.9f x=%.9f", y, y2, x)
			}
		} else {
			// In the GAP: (yMaxNonlin, customKneeX). No exact preimage exists.
			// Our MapReverse should land on the knee (x≈customKneeX), and Map(x)=customKneeX.
			if !almostEqual(y2, customKneeX, eps) {
				t.Fatalf("custom (rev->fwd) gap handling: y=%.9f expected y2≈knee(%.6f) got %.9f (x=%.9f, yMaxNonlin=%.9f)",
					y, customKneeX, y2, x, yMaxNonlin)
			}
		}
	}
}

func TestCustomMapper_MonotoneAndRange(t *testing.T) {
	m := CustomMapper{}
	xs := linspace(4001)
	const eps = 1e-12

	prev := -1.0
	for _, x := range xs {
		y := m.Map(x)
		if !in01(y) || math.IsNaN(y) || math.IsInf(y, 0) {
			t.Fatalf("custom Map out of range/NaN at x=%.9f: %.9f", x, y)
		}
		if y < prev-eps {
			t.Fatalf("custom Map not monotone: y(x=%.6f)=%.9f < prev %.9f", x, y, prev)
		}
		prev = y
	}

	// Endpoints
	if !almostEqual(m.Map(0), 0, 1e-12) {
		t.Fatalf("custom Map(0) != 0: %.12f", m.Map(0))
	}
	if !almostEqual(m.Map(1), 1, 1e-12) {
		t.Fatalf("custom Map(1) != 1: %.12f", m.Map(1))
	}
}

func TestCustomMapper_KneeDiscontinuity_IsAsDesigned(t *testing.T) {
	// Validate the intended C^0 jump at x = customKneeX.
	m := CustomMapper{}
	yBelow := customForwardNonlinear(customKneeX) // note: this uses exact knee; slightly below is nearly identical
	yAbove := m.Map(customKneeX)                  // piecewise uses identity from knee upward → = knee

	if !(yBelow < customKneeX) {
		t.Fatalf("expected yBelow < knee; got yBelow=%.9f knee=%.9f", yBelow, customKneeX)
	}
	if !almostEqual(yAbove, customKneeX, 0) {
		t.Fatalf("expected Map(knee)=knee; got %.9f vs %.9f", yAbove, customKneeX)
	}
}

// ---------- SetCurrentMapper coverage ----------

func TestSetCurrentMapper_SelectsMappers(t *testing.T) {
	SetCurrentMapper("linear")
	if CurrentMapper.String() != "LinearMapper" {
		t.Fatalf("expected LinearMapper, got %s", CurrentMapper.String())
	}
	SetCurrentMapper("custom")
	if CurrentMapper.String() != "CustomMapper" {
		t.Fatalf("expected CustomMapper, got %s", CurrentMapper.String())
	}
	SetCurrentMapper("exp3")
	if CurrentMapper.String() != "ExpMapper^3" {
		t.Fatalf("expected ExpMapper^3, got %s", CurrentMapper.String())
	}
	// bad exp should fall back to linear
	SetCurrentMapper("expNotAnInt")
	if CurrentMapper.String() != "LinearMapper" {
		t.Fatalf("expected fallback LinearMapper on bad exp, got %s", CurrentMapper.String())
	}
}

// ---------- Fuzz (Go 1.18+) ----------

func FuzzCustomMapper_ReverseThenForward(f *testing.F) {
	m := CustomMapper{}
	// Seeds around critical regions
	seeds := []float64{0, 1e-12, 0.05, 0.3, 0.5, 0.69, 0.695, 0.699,
		customKneeX - 1e-9, yMaxNonlinearAtKnee(), customKneeX, 0.9, 1.0}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, y float64) {
		// Clamp to [0,1] because the public API is defined on that domain
		if y < 0 {
			y = 0
		} else if y > 1 {
			y = 1
		}
		x := m.MapReverse(y)
		y2 := m.Map(x)
		yMaxNonlin := yMaxNonlinearAtKnee()
		const eps = 1e-6

		switch {
		case y < yMaxNonlin-1e-9:
			if !almostEqual(y2, y, eps) {
				t.Fatalf("fuzz reachable: y=%.12f -> x=%.12f -> y2=%.12f", y, x, y2)
			}
		case y >= customKneeX:
			if !almostEqual(y2, y, eps) || !almostEqual(x, y, eps) {
				t.Fatalf("fuzz linear: y=%.12f -> x=%.12f -> y2=%.12f", y, x, y2)
			}
		default:
			if !almostEqual(y2, customKneeX, eps) {
				t.Fatalf("fuzz gap: y=%.12f -> x=%.12f -> y2=%.12f (expected knee %.6f)", y, x, y2, customKneeX)
			}
		}
	})
}

var benchSink float64 // avoid dead-code elimination

func BenchmarkCustomMapper_Map(b *testing.B) {
	m := CustomMapper{}
	x := 0.0001
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// walk through the domain without branching noise
		x += 1e-5
		if x >= 1 {
			x = 0.0001
		}
		benchSink = m.Map(x)
	}
}

func BenchmarkCustomMapper_MapReverse(b *testing.B) {
	m := CustomMapper{}

	// Precompute inputs: uniform + knee-biased mix (power-of-two length for fast masking)
	const n = 8192
	ys := make([]float64, n)
	for i := 0; i < n; i++ {
		t := float64(i) / float64(n-1)
		if i%2 == 0 {
			ys[i] = t // uniform coverage [0..1]
		} else {
			// bias half the samples around the knee within ±0.1
			ys[i] = customKneeX + (t-0.5)*0.2
			if ys[i] < 0 {
				ys[i] = 0
			} else if ys[i] > 1 {
				ys[i] = 1
			}
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		y := ys[i&(n-1)] // cheap modulo
		benchSink = m.MapReverse(y)
	}
}
