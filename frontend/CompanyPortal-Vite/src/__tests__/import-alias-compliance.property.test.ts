// Feature: frontend-consolidation, Property 12: Test Import Path Alias Compliance

import * as fs from "node:fs";
import * as path from "node:path";
import * as fc from "fast-check";
import { describe, expect, it } from "vitest";

/**
 * Property 12: Test Import Path Alias Compliance
 * **Validates: Requirements 13.1**
 *
 * For any test file (*.test.ts, *.test.tsx) within ported feature __tests__/ directories,
 * all import statements referencing project source files SHALL use the @/ path alias prefix
 * rather than relative paths reaching outside the feature directory.
 *
 * - Relative imports within the same feature (e.g., ../types) are acceptable.
 * - Cross-feature imports using relative paths (e.g., ../../other-feature/) must use @/ prefix.
 * - Third-party imports (no path prefix like ./ or ../) are always fine.
 */

const FEATURES_DIR = path.resolve(__dirname, "../features");

/**
 * Recursively finds all test files in __tests__/ directories within src/features/
 */
function findTestFiles(): string[] {
  const testFiles: string[] = [];

  if (!fs.existsSync(FEATURES_DIR)) {
    return testFiles;
  }

  const features = fs.readdirSync(FEATURES_DIR, { withFileTypes: true });

  for (const feature of features) {
    if (!feature.isDirectory()) continue;

    const testsDir = path.join(FEATURES_DIR, feature.name, "__tests__");
    if (!fs.existsSync(testsDir)) continue;

    const files = fs.readdirSync(testsDir, { withFileTypes: true });
    for (const file of files) {
      if (file.isFile() && /\.(test|spec)\.(ts|tsx)$/.test(file.name)) {
        testFiles.push(path.join(testsDir, file.name));
      }
    }
  }

  return testFiles;
}

/**
 * Extracts import paths from file content.
 * Matches:
 *   import ... from "path"
 *   import ... from 'path'
 *   import "path"
 *   import 'path'
 */
function extractImportPaths(content: string): string[] {
  const importRegex = /(?:^|\n)\s*import\s+(?:.*?\s+from\s+)?['"]([^'"]+)['"]/g;
  const paths: string[] = [];
  let match: RegExpExecArray | null;

  while ((match = importRegex.exec(content)) !== null) {
    const importPath = match[1];
    if (importPath) {
      paths.push(importPath);
    }
  }

  return paths;
}

/**
 * Determines if a relative import path escapes the feature directory.
 *
 * Test files are at src/features/{featureName}/__tests__/file.test.ts
 * So from __tests__/, going up one level (../) stays within the feature.
 * Going up two levels (../../) exits the feature directory.
 */
function isRelativeImportExitingFeature(importPath: string): boolean {
  // Only check relative paths
  if (!importPath.startsWith(".")) return false;

  // Normalize the path segments
  const segments = importPath.split("/");
  let depth = 0;

  for (const segment of segments) {
    if (segment === "..") {
      depth++;
    } else if (segment !== ".") {
      break;
    }
  }

  // From __tests__/ directory:
  // - depth 1 (../) = still in the same feature directory
  // - depth 2+ (../../) = exits the feature directory into features/ or beyond
  return depth >= 2;
}

/**
 * Checks if an import path is a project source file reference (not a third-party module).
 * Third-party modules don't start with . or / and don't use @/ prefix.
 */
function isProjectSourceImport(importPath: string): boolean {
  // Relative imports are always project source
  if (importPath.startsWith(".")) return true;
  // @/ prefix imports are project source
  if (importPath.startsWith("@/")) return true;
  // Everything else is a third-party/node module
  return false;
}

const testFiles = findTestFiles();

describe("Property 12: Test Import Path Alias Compliance", () => {
  it("all test files in feature __tests__/ directories are discovered", () => {
    // Sanity check: we should find at least some test files
    expect(testFiles.length).toBeGreaterThan(0);
  });

  it("no test file uses relative imports that exit the feature directory", () => {
    fc.assert(
      fc.property(
        fc.constantFrom(...testFiles),
        (testFilePath) => {
          const content = fs.readFileSync(testFilePath, "utf-8");
          const importPaths = extractImportPaths(content);

          const violatingImports = importPaths.filter(
            (p) => isProjectSourceImport(p) && isRelativeImportExitingFeature(p),
          );

          if (violatingImports.length > 0) {
            const relativePath = path.relative(FEATURES_DIR, testFilePath);
            expect.fail(
              `File "${relativePath}" has cross-feature relative imports that should use @/ prefix:\n` +
                violatingImports.map((p) => `  - ${p}`).join("\n"),
            );
          }
        },
      ),
      { numRuns: Math.max(100, testFiles.length * 10) },
    );
  });

  it("cross-feature references use @/ prefix consistently", () => {
    // Verify every test file: imports going to lib/, components/, data/ etc. use @/ prefix
    for (const testFilePath of testFiles) {
      const content = fs.readFileSync(testFilePath, "utf-8");
      const importPaths = extractImportPaths(content);

      for (const importPath of importPaths) {
        if (!isProjectSourceImport(importPath)) continue;

        if (isRelativeImportExitingFeature(importPath)) {
          const relativePath = path.relative(FEATURES_DIR, testFilePath);
          expect.fail(
            `File "${relativePath}" imports "${importPath}" using a relative path ` +
              `that exits the feature directory. It should use the @/ prefix instead.`,
          );
        }
      }
    }
  });
});
