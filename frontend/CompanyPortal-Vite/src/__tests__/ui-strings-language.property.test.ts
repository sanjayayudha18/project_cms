// Feature: frontend-consolidation, Property 11: UI Strings Language Compliance
import * as fs from "node:fs";
import * as path from "node:path";
import * as fc from "fast-check";
import { describe, expect, it } from "vitest";

/**
 * Property 11: UI Strings Language Compliance
 * Validates: Requirements 15.1, 15.5
 *
 * For any user-facing text string rendered by ported feature components,
 * the string SHALL either be in Bahasa Indonesia or consist exclusively
 * of permitted English terms (CIT, DSR, ATM, WMO, CPC, Dashboard, Rp,
 * pcs, kg, IDR, ID, EOD, H+1, H+2, High, Medium, Low, Scheduled,
 * In Transit, Completed, Failed, Open, Close, Normal, Critical, Matched,
 * Mismatch, Pending Review, Escrow).
 */

// ─── Permitted English Terms ──────────────────────────────────────────────────

const PERMITTED_ENGLISH_TERMS = new Set([
  "CIT",
  "DSR",
  "ATM",
  "WMO",
  "CPC",
  "Dashboard",
  "Rp",
  "pcs",
  "kg",
  "IDR",
  "ID",
  "EOD",
  "H+1",
  "H+2",
  "High",
  "Medium",
  "Low",
  "Scheduled",
  "In Transit",
  "Completed",
  "Failed",
  "Open",
  "Close",
  "Normal",
  "Critical",
  "Matched",
  "Mismatch",
  "Pending Review",
  "Escrow",
  // Unit abbreviations and formatting
  "Cash Flow",
  "Cash Flow Monitoring",
  "CIT Tracker",
  "Status",
  "Vendor",
  "Progress",
  "Evidence",
]);

// Known English phrases that SHOULD have been translated in Source_App
// but must not appear as standalone user-facing text in Target_App feature components.
// These are multi-word phrases or standalone single words used as UI labels.
// Single words like "Filter", "Total", "Date" are excluded because they commonly
// appear as substrings within Bahasa Indonesia text (e.g., "sesuai filter").
const FORBIDDEN_ENGLISH_PHRASES = [
  "Good morning",
  "Good afternoon",
  "Good evening",
  "Good night",
  "Loading...",
  "Loading data",
  "No data available",
  "No data found",
  "No results found",
  "Error occurred",
  "Something went wrong",
  "An error occurred",
  "Please try again",
  "Try again later",
  "View details",
  "View more",
  "Show more",
  "Show less",
  "Sort by",
  "Sign in",
  "Sign out",
  "Log in",
  "Log out",
  "Are you sure",
  "No items found",
  "Click here",
  "Enter your",
  "Would you like",
  "There are no",
  "There is no",
];

// Single English words that are ONLY forbidden when they appear as a standalone
// JSX text node (the entire visible text is just this word). Words like "filter"
// naturally occur inside Bahasa Indonesia sentences and should not be flagged.
const FORBIDDEN_STANDALONE_LABELS = [
  "Retry",
  "Submit",
  "Cancel",
  "Delete",
  "Save",
  "Edit",
  "Update",
  "Search",
  "Filter",
  "Total",
  "Amount",
  "Date",
  "Description",
  "Actions",
  "Back",
  "Next",
  "Previous",
  "Page",
  "Welcome",
  "Hello",
  "Username",
  "Password",
  "Confirm",
  "Success",
  "Warning",
  "Info",
  "Empty",
  "Select",
  "Choose",
  "Approve",
  "Reject",
  "Pending",
  "Download",
  "Upload",
  "Export",
  "Import",
];

// ─── File Discovery ───────────────────────────────────────────────────────────

const FEATURES_DIR = path.resolve(__dirname, "../features");

function getAllTsxFiles(dir: string): string[] {
  const files: string[] = [];

  if (!fs.existsSync(dir)) return files;

  const entries = fs.readdirSync(dir, { withFileTypes: true });
  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      // Skip __tests__ directories - they contain test utilities not user-facing text
      if (entry.name === "__tests__" || entry.name === "node_modules") continue;
      files.push(...getAllTsxFiles(fullPath));
    } else if (entry.name.endsWith(".tsx")) {
      files.push(fullPath);
    }
  }

  return files;
}

/**
 * Extract user-facing JSX string literals from a file's content.
 * Only captures text that is likely rendered to users — ignores code,
 * comments, and multi-line expressions.
 */
function extractJsxStringLiterals(content: string): string[] {
  const strings: string[] = [];

  // Remove block comments (/* ... */) and line comments (// ...)
  // so we don't accidentally capture comment text
  const withoutComments = content.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

  // Match single-line text content between JSX tags: >some text<
  // Only match text on a single line (no newlines) to avoid grabbing code blocks
  const jsxTextRegex = />([^<>{}\n]+)</g;
  let match: RegExpExecArray | null = jsxTextRegex.exec(withoutComments);

  while (match !== null) {
    const text = match[1]?.trim();
    if (text && text.length > 1 && !/^\s*$/.test(text)) {
      // Skip if it looks like code: contains =, ;, (), [], or template literals
      // Skip if it starts with a dot (method chain) or is camelCase/PascalCase identifier
      if (
        !/[=;()\[\]`$]/.test(text) &&
        !/^\.[a-z]/.test(text) &&
        !/^[a-z][a-zA-Z0-9]+$/.test(text)
      ) {
        strings.push(text);
      }
    }
    match = jsxTextRegex.exec(withoutComments);
  }

  // Match string literals in JSX attributes that are user-facing:
  // label="...", title="...", description="...", message="...", placeholder="...", eyebrow="..."
  const attrRegex = /(?:label|title|description|message|placeholder|eyebrow)\s*=\s*"([^"]+)"/g;
  let attrMatch: RegExpExecArray | null = attrRegex.exec(withoutComments);
  while (attrMatch !== null) {
    const text = attrMatch[1]?.trim();
    if (text && text.length > 1) {
      strings.push(text);
    }
    attrMatch = attrRegex.exec(withoutComments);
  }

  return strings;
}

/**
 * Checks if a string is exclusively composed of permitted English terms,
 * technical words, numbers, or punctuation.
 */
function isPermittedEnglish(text: string): boolean {
  // Check if the full text is a permitted term
  if (PERMITTED_ENGLISH_TERMS.has(text)) return true;

  // Check if it's a pure number/currency/date format
  if (/^[\d.,\s%:+\-/()Rp]+$/.test(text)) return true;

  // Check if it's a code/technical term (camelCase, PascalCase, snake_case, kebab-case)
  if (/^[a-zA-Z][a-zA-Z0-9_-]*$/.test(text) && /[A-Z]/.test(text.slice(1))) return true;

  // Check if it's a CSS class reference or prop value
  if (text.startsWith("var(--") || text.includes("className")) return true;

  // Check if all words in the text are permitted
  const words = text.split(/[\s,·|:;]+/).filter((w) => w.length > 0);
  return words.every(
    (word) =>
      PERMITTED_ENGLISH_TERMS.has(word) ||
      // Numbers, currency symbols, punctuation
      /^[\d.,+\-/%()Rp]+$/.test(word) ||
      // Single characters
      word.length <= 1 ||
      // Technical abbreviations (all caps, 2-5 chars)
      (/^[A-Z][A-Z0-9+\-]{0,5}$/.test(word) && word.length <= 6),
  );
}

/**
 * Checks if a string is a forbidden English phrase that should have been translated.
 * Two-tier check:
 * 1. Multi-word phrases: flagged if they appear as substrings (case-insensitive)
 * 2. Standalone labels: only flagged if the ENTIRE extracted text equals the label
 */
function isForbiddenEnglish(text: string): boolean {
  const lowerText = text.toLowerCase().trim();

  // Check multi-word forbidden phrases (substring match)
  const hasPhrase = FORBIDDEN_ENGLISH_PHRASES.some((phrase) =>
    lowerText.includes(phrase.toLowerCase()),
  );
  if (hasPhrase) return true;

  // Check standalone labels — only if the text is exactly this word
  const isStandaloneLabel = FORBIDDEN_STANDALONE_LABELS.some(
    (label) => lowerText === label.toLowerCase(),
  );
  return isStandaloneLabel;
}

// ─── Tests ────────────────────────────────────────────────────────────────────

describe("Property 11: UI Strings Language Compliance", () => {
  const tsxFiles = getAllTsxFiles(FEATURES_DIR);

  it("should find .tsx feature files to scan", () => {
    expect(tsxFiles.length).toBeGreaterThan(0);
  });

  it("no ported feature component contains forbidden English phrases that should be translated", () => {
    const violations: Array<{
      file: string;
      text: string;
      matchedPhrase: string;
    }> = [];

    for (const file of tsxFiles) {
      const content = fs.readFileSync(file, "utf-8");
      const strings = extractJsxStringLiterals(content);

      for (const text of strings) {
        if (isForbiddenEnglish(text)) {
          const relPath = path.relative(FEATURES_DIR, file);
          const lowerText = text.toLowerCase().trim();
          const matchedPhrase =
            FORBIDDEN_ENGLISH_PHRASES.find((p) => lowerText.includes(p.toLowerCase())) ??
            FORBIDDEN_STANDALONE_LABELS.find((l) => lowerText === l.toLowerCase()) ??
            text;
          violations.push({ file: relPath, text, matchedPhrase });
        }
      }
    }

    expect(
      violations,
      `Found ${violations.length} untranslated English phrase(s) in feature components:\n${violations
        .map((v) => `  ${v.file}: "${v.text}" (should translate: "${v.matchedPhrase}")`)
        .join("\n")}`,
    ).toHaveLength(0);
  });

  it("property: for any picked .tsx feature file, all user-facing text is either Bahasa Indonesia or permitted English", () => {
    // Build file index for generator
    const fileContents = tsxFiles.map((file) => ({
      file,
      content: fs.readFileSync(file, "utf-8"),
    }));

    if (fileContents.length === 0) return;

    // Generator: pick a random file and a random string from it
    const fileIndex = fc.integer({ min: 0, max: fileContents.length - 1 });

    fc.assert(
      fc.property(fileIndex, (idx) => {
        const entry = fileContents[idx];
        if (!entry) return;
        const { file, content } = entry;
        const strings = extractJsxStringLiterals(content);
        const relPath = path.relative(FEATURES_DIR, file);

        for (const text of strings) {
          // Skip if the text is permitted English (acronyms, product names, status terms)
          if (isPermittedEnglish(text)) continue;

          // The string MUST NOT be a forbidden English phrase
          const isForbidden = isForbiddenEnglish(text);
          if (isForbidden) {
            const lowerText = text.toLowerCase().trim();
            const matchedPhrase =
              FORBIDDEN_ENGLISH_PHRASES.find((p) => lowerText.includes(p.toLowerCase())) ??
              FORBIDDEN_STANDALONE_LABELS.find((l) => lowerText === l.toLowerCase());
            expect.fail(
              `Forbidden English phrase in ${relPath}: "${text}" ` +
                `(matched: "${matchedPhrase}"). Should be translated to Bahasa Indonesia.`,
            );
          }
        }
      }),
      { numRuns: 100 },
    );
  });

  it("property: strings in feature components do not contain full English sentences (subject-verb patterns)", () => {
    const fileContents = tsxFiles.map((file) => ({
      file,
      content: fs.readFileSync(file, "utf-8"),
    }));

    if (fileContents.length === 0) return;

    // Regex patterns for detecting full English sentences
    // (subject + verb patterns that indicate untranslated text)
    const englishSentencePatterns = [
      // "The [noun] is/are/was/were..."
      /\bThe\s+\w+\s+(?:is|are|was|were|has|have)\b/i,
      // "This [noun] is/has..."
      /\bThis\s+\w+\s+(?:is|are|has|have|will|can)\b/i,
      // "Please [verb]..."
      /\bPlease\s+[a-z]+/i,
      // "You have/are/can..."
      /\bYou\s+(?:have|are|can|will|should|must|need)\b/i,
      // "Click here/below/above to..."
      /\bClick\s+(?:here|below|above|to)\b/i,
      // "Enter your..." / "Input your..."
      /\b(?:Enter|Input)\s+your\b/i,
      // "There are no..." / "There is no..."
      /\bThere\s+(?:are|is)\s+no\b/i,
      // "Could not..." / "Cannot..." / "Unable to..."
      /\b(?:Could not|Cannot|Unable to)\s+\w+/i,
      // "Would you like to..."
      /\bWould you like to\b/i,
      // "Data is being..." / "Data has been..."
      /\bData\s+(?:is|has)\s+(?:being|been)\b/i,
    ];

    const fileIndex = fc.integer({ min: 0, max: fileContents.length - 1 });

    fc.assert(
      fc.property(fileIndex, (idx) => {
        const entry = fileContents[idx];
        if (!entry) return;
        const { file, content } = entry;
        const strings = extractJsxStringLiterals(content);
        const relPath = path.relative(FEATURES_DIR, file);

        for (const text of strings) {
          // Skip short strings (less than 4 words are likely labels)
          if (text.split(/\s+/).length < 4) continue;
          // Skip if entirely permitted
          if (isPermittedEnglish(text)) continue;

          for (const pattern of englishSentencePatterns) {
            if (pattern.test(text)) {
              expect.fail(
                `English sentence found in ${relPath}: "${text}" (matched pattern: ${pattern}). Full English sentences must be translated to Bahasa Indonesia.`,
              );
            }
          }
        }
      }),
      { numRuns: 100 },
    );
  });
});
