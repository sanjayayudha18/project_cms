// Feature: frontend-consolidation, Property 10: No Hardcoded Color Values in Ported Files
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import * as fc from "fast-check";
import { describe, expect, it } from "vitest";

/**
 * Property 10: No Hardcoded Color Values in Ported Files
 * Validates: Requirements 15.2
 *
 * For any .tsx or .ts file within src/features/ directories of ported modules,
 * the file content SHALL contain zero matches for hardcoded hex color patterns
 * (#[0-9a-fA-F]{3,8}), RGB patterns (rgb( or rgba(), or HSL patterns (hsl( or hsla().
 *
 * EXCEPTIONS:
 * - oklch() values are allowed (design system uses OKLCH color space)
 * - Test files (__tests__/ directories) are excluded from scanning
 */

// Patterns for disallowed hardcoded color values
const HEX_COLOR_PATTERN = /#[0-9a-fA-F]{3,8}\b/g;
const RGB_PATTERN = /\brgba?\s*\(/g;
const HSL_PATTERN = /\bhsla?\s*\(/g;

/**
 * Recursively collect all .ts and .tsx files in a directory,
 * excluding __tests__/ directories and test files.
 */
function collectSourceFiles(dir: string): string[] {
  const files: string[] = [];

  function walk(currentDir: string): void {
    let entries: string[];
    try {
      entries = readdirSync(currentDir);
    } catch {
      return;
    }

    for (const entry of entries) {
      const fullPath = join(currentDir, entry);

      // Skip __tests__ directories
      if (entry === "__tests__") continue;

      let stat: ReturnType<typeof statSync>;
      try {
        stat = statSync(fullPath);
      } catch {
        continue;
      }

      if (stat.isDirectory()) {
        walk(fullPath);
      } else if (stat.isFile() && /\.(ts|tsx)$/.test(entry)) {
        // Exclude test files by name pattern
        if (/\.(test|spec)\.(ts|tsx)$/.test(entry)) continue;
        files.push(fullPath);
      }
    }
  }

  walk(dir);
  return files;
}

/**
 * Strip content that should not trigger false positives:
 * - Single-line comments (// ...)
 * - Multi-line comments (/* ... *\/)
 */
function stripComments(content: string): string {
  // Remove single-line comments
  let result = content.replace(/\/\/.*$/gm, "");
  // Remove multi-line comments
  result = result.replace(/\/\*[\s\S]*?\*\//g, "");
  return result;
}

/**
 * Check a file for hardcoded color values.
 * Returns an array of violations found.
 */
function findColorViolations(
  _filePath: string,
  content: string,
): { pattern: string; match: string; line: number }[] {
  const violations: { pattern: string; match: string; line: number }[] = [];
  const strippedContent = stripComments(content);
  const lines = strippedContent.split("\n");

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]!;

    // Check hex colors
    const hexMatches = line.matchAll(HEX_COLOR_PATTERN);
    for (const match of hexMatches) {
      violations.push({ pattern: "hex", match: match[0], line: i + 1 });
    }

    // Check RGB/RGBA
    const rgbMatches = line.matchAll(RGB_PATTERN);
    for (const match of rgbMatches) {
      violations.push({ pattern: "rgb/rgba", match: match[0], line: i + 1 });
    }

    // Check HSL/HSLA
    const hslMatches = line.matchAll(HSL_PATTERN);
    for (const match of hslMatches) {
      violations.push({ pattern: "hsl/hsla", match: match[0], line: i + 1 });
    }
  }

  return violations;
}

// Resolve the features directory path
const FEATURES_DIR = resolve(__dirname, "../features");

// Collect all source files from src/features/
const sourceFiles = collectSourceFiles(FEATURES_DIR);

describe("Property 10: No Hardcoded Color Values in Ported Files", () => {
  it("should find source files in src/features/ to scan", () => {
    expect(sourceFiles.length).toBeGreaterThan(0);
  });

  it("no .ts/.tsx file in src/features/ contains hardcoded hex, RGB, or HSL colors", () => {
    // Use fast-check to randomly sample and verify files
    // We use constantFrom to pick files from the discovered set
    fc.assert(
      fc.property(fc.constantFrom(...sourceFiles), (filePath) => {
        const content = readFileSync(filePath, "utf-8");
        const violations = findColorViolations(filePath, content);

        const relativePath = relative(FEATURES_DIR, filePath);

        if (violations.length > 0) {
          const violationDetails = violations
            .map((v) => `  Line ${v.line}: ${v.pattern} → "${v.match}"`)
            .join("\n");

          expect.fail(
            `File "src/features/${relativePath}" contains ${violations.length} hardcoded color(s):\n${violationDetails}`,
          );
        }
      }),
      { numRuns: Math.max(sourceFiles.length * 3, 100) },
    );
  });

  it("exhaustive check: every source file in src/features/ is free of hardcoded colors", () => {
    // Deterministic scan of ALL files (complements the property-based random sampling)
    const allViolations: { file: string; violations: { pattern: string; match: string; line: number }[] }[] = [];

    for (const filePath of sourceFiles) {
      const content = readFileSync(filePath, "utf-8");
      const violations = findColorViolations(filePath, content);

      if (violations.length > 0) {
        allViolations.push({
          file: `src/features/${relative(FEATURES_DIR, filePath)}`,
          violations,
        });
      }
    }

    if (allViolations.length > 0) {
      const report = allViolations
        .map(
          (v) =>
            `${v.file}:\n${v.violations.map((d) => `  Line ${d.line}: ${d.pattern} → "${d.match}"`).join("\n")}`,
        )
        .join("\n\n");

      expect.fail(
        `Found hardcoded color values in ${allViolations.length} file(s):\n\n${report}`,
      );
    }
  });
});
