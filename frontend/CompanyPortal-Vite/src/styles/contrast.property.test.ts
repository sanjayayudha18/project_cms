import * as fc from "fast-check";
import { describe, expect, it } from "vitest";

/**
 * Property 21: Design System Contrast Compliance
 * Validates: Requirements 10.4
 *
 * For any (text-color-token, background-color-token) pair used in the design system,
 * the computed OKLCH contrast ratio SHALL be ≥ 4.5:1 for normal text sizes
 * (< 18px or < 14px bold) and ≥ 3:1 for large text sizes (≥ 18px or ≥ 14px bold).
 */

// ============================================================
// OKLCH → sRGB → Relative Luminance Conversion Pipeline
// ============================================================

/**
 * Convert OKLCH (Oklab Lightness, Chroma, Hue) to OKLab (L, a, b)
 * Polar → Cartesian conversion
 */
function oklchToOklab(L: number, C: number, H: number): [number, number, number] {
  const hRad = (H * Math.PI) / 180;
  const a = C * Math.cos(hRad);
  const b = C * Math.sin(hRad);
  return [L, a, b];
}

/**
 * Convert OKLab to linear sRGB
 * Based on the OKLab specification by Björn Ottosson
 */
function oklabToLinearSrgb(L: number, a: number, b: number): [number, number, number] {
  // OKLab → LMS (cone response)
  const l_ = L + 0.3963377774 * a + 0.2158037573 * b;
  const m_ = L - 0.1055613458 * a - 0.0638541728 * b;
  const s_ = L - 0.0894841775 * a - 1.291485548 * b;

  // Cube to undo the cube-root nonlinearity
  const l = l_ * l_ * l_;
  const m = m_ * m_ * m_;
  const s = s_ * s_ * s_;

  // LMS → linear sRGB
  const r = +4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s;
  const g = -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s;
  const bl = -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s;

  return [r, g, bl];
}

/**
 * Apply sRGB gamma encoding (linear → sRGB)
 */
function linearToSrgb(c: number): number {
  if (c <= 0.0031308) {
    return 12.92 * c;
  }
  return 1.055 * c ** (1 / 2.4) - 0.055;
}

/**
 * Convert sRGB channel value (0-1) back to linear for luminance calculation.
 * This is the inverse of gamma encoding.
 */
function srgbToLinear(c: number): number {
  if (c <= 0.04045) {
    return c / 12.92;
  }
  return ((c + 0.055) / 1.055) ** 2.4;
}

/**
 * Compute relative luminance from linear sRGB values.
 * Per WCAG 2.1: L = 0.2126 * R + 0.7152 * G + 0.0722 * B
 */
function relativeLuminance(r: number, g: number, b: number): number {
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

/**
 * Compute WCAG contrast ratio between two relative luminance values.
 * Formula: (L1 + 0.05) / (L2 + 0.05) where L1 >= L2
 */
function contrastRatio(lum1: number, lum2: number): number {
  const lighter = Math.max(lum1, lum2);
  const darker = Math.min(lum1, lum2);
  return (lighter + 0.05) / (darker + 0.05);
}

/**
 * Full pipeline: OKLCH values → relative luminance
 */
function oklchToLuminance(L: number, C: number, H: number): number {
  const [labL, labA, labB] = oklchToOklab(L, C, H);
  const [linR, linG, linB] = oklabToLinearSrgb(labL, labA, labB);

  // Clamp to [0, 1] for gamut mapping (OKLCH values may be out of sRGB gamut)
  const clampedR = Math.max(0, Math.min(1, linR));
  const clampedG = Math.max(0, Math.min(1, linG));
  const clampedB = Math.max(0, Math.min(1, linB));

  // Convert to sRGB then back to linear for luminance
  const sR = linearToSrgb(clampedR);
  const sG = linearToSrgb(clampedG);
  const sB = linearToSrgb(clampedB);

  return relativeLuminance(srgbToLinear(sR), srgbToLinear(sG), srgbToLinear(sB));
}

/**
 * Parse an OKLCH CSS string like "oklch(0.552 0.205 29)" into [L, C, H]
 */
function parseOklch(value: string): [number, number, number] {
  const match = value.match(/oklch\(\s*([\d.]+)\s+([\d.]+)\s+([\d.]+)\s*\)/);
  if (!match) {
    throw new Error(`Invalid OKLCH value: ${value}`);
  }
  return [Number.parseFloat(match[1]), Number.parseFloat(match[2]), Number.parseFloat(match[3])];
}

/**
 * Compute WCAG contrast ratio between two OKLCH color strings
 */
function computeContrastFromOklch(fg: string, bg: string): number {
  const [fgL, fgC, fgH] = parseOklch(fg);
  const [bgL, bgC, bgH] = parseOklch(bg);
  const fgLum = oklchToLuminance(fgL, fgC, fgH);
  const bgLum = oklchToLuminance(bgL, bgC, bgH);
  return contrastRatio(fgLum, bgLum);
}

// ============================================================
// Design System Token Definitions
// ============================================================

const TOKENS: Record<string, string> = {
  "--red-500": "oklch(0.552 0.205 29)",
  "--n-0": "oklch(0.992 0.003 29)",
  "--n-50": "oklch(0.975 0.004 29)",
  "--n-500": "oklch(0.560 0.009 29)",
  "--n-600": "oklch(0.448 0.008 29)",
  "--n-700": "oklch(0.352 0.007 29)",
  "--n-800": "oklch(0.258 0.006 29)",
  "--n-900": "oklch(0.178 0.005 29)",
  "--success-bg": "oklch(0.955 0.03 155)",
  "--success-fg": "oklch(0.480 0.115 155)",
  "--warning-bg": "oklch(0.960 0.055 78)",
  "--warning-fg": "oklch(0.520 0.115 78)",
  "--danger-bg": "oklch(0.955 0.035 12)",
  "--danger-fg": "oklch(0.500 0.195 12)",
  "--info-bg": "oklch(0.955 0.03 245)",
  "--info-fg": "oklch(0.480 0.110 245)",
  /* Scoped login shell tokens */
  "--login-chrome-deep": "oklch(0.300 0.110 29)",
  "--login-chrome-fg": "oklch(0.940 0.012 29)",
  "--login-well": "oklch(0.245 0.088 29)",
  "--login-fg-dim": "oklch(0.820 0.045 29)",
  "--login-fg-muted": "oklch(0.680 0.050 29)",
  "--login-primary": "oklch(0.552 0.205 29)",
  "--login-primary-fg": "oklch(0.990 0.004 29)",
  "--login-danger-fg": "oklch(0.820 0.120 12)",
  "--login-danger-bg": "oklch(0.280 0.060 12)",
};

// ============================================================
// Color Pair Definitions with Minimum Ratios
// ============================================================

interface ColorPair {
  name: string;
  fg: string;
  bg: string;
  minRatio: number;
  sizeCategory: "normal" | "large";
}

const NORMAL_TEXT_PAIRS: ColorPair[] = [
  {
    name: "--n-800 on --n-0 (body text on card)",
    fg: TOKENS["--n-800"],
    bg: TOKENS["--n-0"],
    minRatio: 4.5,
    sizeCategory: "normal",
  },
  {
    name: "--n-800 on --n-50 (body text on page)",
    fg: TOKENS["--n-800"],
    bg: TOKENS["--n-50"],
    minRatio: 4.5,
    sizeCategory: "normal",
  },
  {
    name: "--n-900 on --n-0 (headings on card)",
    fg: TOKENS["--n-900"],
    bg: TOKENS["--n-0"],
    minRatio: 4.5,
    sizeCategory: "normal",
  },
  {
    name: "--n-900 on --n-50 (headings on page)",
    fg: TOKENS["--n-900"],
    bg: TOKENS["--n-50"],
    minRatio: 4.5,
    sizeCategory: "normal",
  },
  {
    name: "--n-700 on --n-0 (labels)",
    fg: TOKENS["--n-700"],
    bg: TOKENS["--n-0"],
    minRatio: 4.5,
    sizeCategory: "normal",
  },
  {
    name: "--n-600 on --n-0 (secondary text)",
    fg: TOKENS["--n-600"],
    bg: TOKENS["--n-0"],
    minRatio: 4.5,
    sizeCategory: "normal",
  },
  {
    name: "--n-500 on --n-0 (metadata)",
    fg: TOKENS["--n-500"],
    bg: TOKENS["--n-0"],
    minRatio: 4.5,
    sizeCategory: "normal",
  },
  {
    name: "--red-500 on --n-0 (primary button text uses)",
    fg: TOKENS["--red-500"],
    bg: TOKENS["--n-0"],
    minRatio: 4.5,
    sizeCategory: "normal",
  },
  {
    name: "--danger-fg on --danger-bg (error text)",
    fg: TOKENS["--danger-fg"],
    bg: TOKENS["--danger-bg"],
    minRatio: 4.5,
    sizeCategory: "normal",
  },
  {
    name: "--success-fg on --success-bg (success text)",
    fg: TOKENS["--success-fg"],
    bg: TOKENS["--success-bg"],
    minRatio: 4.5,
    sizeCategory: "normal",
  },
  {
    name: "--warning-fg on --warning-bg (warning text)",
    fg: TOKENS["--warning-fg"],
    bg: TOKENS["--warning-bg"],
    minRatio: 4.5,
    sizeCategory: "normal",
  },
  {
    name: "--info-fg on --info-bg (info text)",
    fg: TOKENS["--info-fg"],
    bg: TOKENS["--info-bg"],
    minRatio: 4.5,
    sizeCategory: "normal",
  },
  {
    name: "--login-chrome-fg on --login-chrome-deep (login body)",
    fg: TOKENS["--login-chrome-fg"],
    bg: TOKENS["--login-chrome-deep"],
    minRatio: 4.5,
    sizeCategory: "normal",
  },
  {
    name: "--login-chrome-fg on --login-well (login input text)",
    fg: TOKENS["--login-chrome-fg"],
    bg: TOKENS["--login-well"],
    minRatio: 4.5,
    sizeCategory: "normal",
  },
  {
    name: "--login-fg-dim on --login-chrome-deep (login secondary)",
    fg: TOKENS["--login-fg-dim"],
    bg: TOKENS["--login-chrome-deep"],
    minRatio: 4.5,
    sizeCategory: "normal",
  },
  {
    name: "--login-danger-fg on --login-danger-bg (login alert)",
    fg: TOKENS["--login-danger-fg"],
    bg: TOKENS["--login-danger-bg"],
    minRatio: 4.5,
    sizeCategory: "normal",
  },
];

const LARGE_TEXT_PAIRS: ColorPair[] = [
  {
    name: "--red-500 on --n-0 (active nav item, large)",
    fg: TOKENS["--red-500"],
    bg: TOKENS["--n-0"],
    minRatio: 3.0,
    sizeCategory: "large",
  },
  {
    name: "--n-0 on --red-500 (primary button white text)",
    fg: TOKENS["--n-0"],
    bg: TOKENS["--red-500"],
    minRatio: 3.0,
    sizeCategory: "large",
  },
  {
    name: "--login-primary-fg on --login-primary (login CTA)",
    fg: TOKENS["--login-primary-fg"],
    bg: TOKENS["--login-primary"],
    minRatio: 3.0,
    sizeCategory: "large",
  },
];

const ALL_PAIRS = [...NORMAL_TEXT_PAIRS, ...LARGE_TEXT_PAIRS];

// ============================================================
// Tests
// ============================================================

describe("Property 21: Design System Contrast Compliance", () => {
  // --- Static pair verification ---

  describe("Normal text pairs (≥ 4.5:1 required)", () => {
    for (const pair of NORMAL_TEXT_PAIRS) {
      it(`${pair.name} meets 4.5:1 contrast ratio`, () => {
        const ratio = computeContrastFromOklch(pair.fg, pair.bg);
        expect(ratio).toBeGreaterThanOrEqual(pair.minRatio);
      });
    }
  });

  describe("Large text / UI component pairs (≥ 3:1 required)", () => {
    for (const pair of LARGE_TEXT_PAIRS) {
      it(`${pair.name} meets 3:1 contrast ratio`, () => {
        const ratio = computeContrastFromOklch(pair.fg, pair.bg);
        expect(ratio).toBeGreaterThanOrEqual(pair.minRatio);
      });
    }
  });

  // --- Property-based tests for contrast function correctness ---

  describe("Contrast ratio mathematical properties", () => {
    const arbLuminance = fc.double({ min: 0, max: 1, noNaN: true });

    it("contrastRatio is commutative: ratio(A, B) === ratio(B, A)", () => {
      fc.assert(
        fc.property(arbLuminance, arbLuminance, (lum1, lum2) => {
          const ratio1 = contrastRatio(lum1, lum2);
          const ratio2 = contrastRatio(lum2, lum1);
          expect(ratio1).toBeCloseTo(ratio2, 10);
        }),
      );
    });

    it("contrastRatio is always >= 1 (minimum ratio is 1:1 for identical colors)", () => {
      fc.assert(
        fc.property(arbLuminance, arbLuminance, (lum1, lum2) => {
          const ratio = contrastRatio(lum1, lum2);
          expect(ratio).toBeGreaterThanOrEqual(1);
        }),
      );
    });

    it("contrastRatio of identical luminance is exactly 1", () => {
      fc.assert(
        fc.property(arbLuminance, (lum) => {
          const ratio = contrastRatio(lum, lum);
          expect(ratio).toBeCloseTo(1, 10);
        }),
      );
    });

    it("contrastRatio maximum is 21:1 (black vs white)", () => {
      const ratio = contrastRatio(1, 0);
      expect(ratio).toBeCloseTo(21, 1);
    });

    it("darker foreground always has higher ratio against pure white", () => {
      fc.assert(
        fc.property(
          fc.double({ min: 0, max: 0.49, noNaN: true }),
          fc.double({ min: 0.5, max: 1, noNaN: true }),
          (darkerLum, lighterLum) => {
            const ratioDark = contrastRatio(darkerLum, 1);
            const ratioLight = contrastRatio(lighterLum, 1);
            expect(ratioDark).toBeGreaterThanOrEqual(ratioLight);
          },
        ),
      );
    });
  });

  describe("OKLCH conversion edge cases", () => {
    it("oklchToLuminance produces values in [0, 1] range for any valid OKLCH input", () => {
      const arbOklch = fc.tuple(
        fc.double({ min: 0, max: 1, noNaN: true }), // L: lightness
        fc.double({ min: 0, max: 0.4, noNaN: true }), // C: chroma (realistic range)
        fc.double({ min: 0, max: 360, noNaN: true }), // H: hue
      );

      fc.assert(
        fc.property(arbOklch, ([L, C, H]) => {
          const lum = oklchToLuminance(L, C, H);
          expect(lum).toBeGreaterThanOrEqual(0);
          expect(lum).toBeLessThanOrEqual(1);
        }),
      );
    });

    it("oklchToLuminance(0, 0, 0) produces near-zero luminance (black)", () => {
      const lum = oklchToLuminance(0, 0, 0);
      expect(lum).toBeCloseTo(0, 2);
    });

    it("oklchToLuminance(1, 0, 0) produces near-maximum luminance (white)", () => {
      const lum = oklchToLuminance(1, 0, 0);
      expect(lum).toBeCloseTo(1, 2);
    });

    it("parseOklch correctly extracts L, C, H from valid OKLCH strings", () => {
      const arbL = fc.double({ min: 0, max: 1, noNaN: true });
      const arbC = fc.double({ min: 0, max: 0.4, noNaN: true });
      const arbH = fc.double({ min: 0, max: 360, noNaN: true });

      fc.assert(
        fc.property(arbL, arbC, arbH, (L, C, H) => {
          // Use fixed precision to avoid floating-point formatting issues
          const lStr = L.toFixed(3);
          const cStr = C.toFixed(3);
          const hStr = H.toFixed(1);
          const oklchStr = `oklch(${lStr} ${cStr} ${hStr})`;
          const [parsedL, parsedC, parsedH] = parseOklch(oklchStr);
          expect(parsedL).toBeCloseTo(L, 2);
          expect(parsedC).toBeCloseTo(C, 2);
          expect(parsedH).toBeCloseTo(H, 0);
        }),
      );
    });
  });

  describe("All design system pairs meet minimum contrast", () => {
    it("every defined color pair from the design system passes its required minimum contrast ratio", () => {
      const arbPairIndex = fc.integer({ min: 0, max: ALL_PAIRS.length - 1 });

      fc.assert(
        fc.property(arbPairIndex, (idx) => {
          const pair = ALL_PAIRS[idx];
          const ratio = computeContrastFromOklch(pair.fg, pair.bg);
          expect(ratio).toBeGreaterThanOrEqual(pair.minRatio);
        }),
      );
    });
  });
});
