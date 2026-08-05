import * as fc from 'fast-check';

/**
 * Property 9: File Upload Validation
 * Validates: Requirements 4.2
 *
 * For any file metadata (type and size): if the file type is one of
 * [image/jpeg, image/png, application/pdf] AND file size is ≤ 10,485,760 bytes (10MB),
 * validation should pass. For any file with type not in the accepted list OR
 * size exceeding 10MB, validation should fail with a specific error message
 * identifying the constraint violated.
 */

const ACCEPTED_TYPES = ['image/jpeg', 'image/png', 'application/pdf'];
const MAX_SIZE_BYTES = 10_485_760; // 10MB

interface FileValidationResult {
  valid: boolean;
  error?: string;
}

function validateFile(
  type: string,
  size: number,
  fileName: string,
): FileValidationResult {
  if (!ACCEPTED_TYPES.includes(type)) {
    return {
      valid: false,
      error: `File ${fileName} tidak didukung. Gunakan JPEG, PNG, atau PDF.`,
    };
  }
  if (size > MAX_SIZE_BYTES) {
    return { valid: false, error: `File ${fileName} melebihi batas 10MB` };
  }
  return { valid: true };
}

// Generators
const validTypeArb = fc.constantFrom(
  'image/jpeg',
  'image/png',
  'application/pdf',
);
const invalidTypeArb = fc.constantFrom(
  'application/exe',
  'text/plain',
  'application/zip',
  'video/mp4',
  'application/x-msdownload',
);
const validSizeArb = fc.integer({ min: 1, max: 10_485_760 });
const invalidSizeArb = fc.integer({ min: 10_485_761, max: 100_000_000 });
const fileNameArb = fc
  .string({ minLength: 1, maxLength: 50 })
  .map((s) => s.replace(/[^a-zA-Z0-9._-]/g, 'x') + '.file');

describe('Property 9: File Upload Validation', () => {
  it('files with accepted type AND size ≤ 10MB pass validation', () => {
    fc.assert(
      fc.property(validTypeArb, validSizeArb, fileNameArb, (type, size, name) => {
        const result = validateFile(type, size, name);
        expect(result.valid).toBe(true);
        expect(result.error).toBeUndefined();
      }),
      { numRuns: 100 },
    );
  });

  it('files with invalid type fail with "tidak didukung" error', () => {
    fc.assert(
      fc.property(
        invalidTypeArb,
        validSizeArb,
        fileNameArb,
        (type, size, name) => {
          const result = validateFile(type, size, name);
          expect(result.valid).toBe(false);
          expect(result.error).toContain('tidak didukung');
          expect(result.error).toContain('Gunakan JPEG, PNG, atau PDF');
          expect(result.error).toContain(name);
        },
      ),
      { numRuns: 100 },
    );
  });

  it('files with size > 10MB fail with "melebihi batas" error', () => {
    fc.assert(
      fc.property(
        validTypeArb,
        invalidSizeArb,
        fileNameArb,
        (type, size, name) => {
          const result = validateFile(type, size, name);
          expect(result.valid).toBe(false);
          expect(result.error).toContain('melebihi batas 10MB');
          expect(result.error).toContain(name);
        },
      ),
      { numRuns: 100 },
    );
  });

  it('files with both invalid type AND size > 10MB fail validation (either error acceptable)', () => {
    fc.assert(
      fc.property(
        invalidTypeArb,
        invalidSizeArb,
        fileNameArb,
        (type, size, name) => {
          const result = validateFile(type, size, name);
          expect(result.valid).toBe(false);
          expect(result.error).toBeDefined();
          // Either error message is acceptable — the function checks type first
          const hasTypeError = result.error!.includes('tidak didukung');
          const hasSizeError = result.error!.includes('melebihi batas');
          expect(hasTypeError || hasSizeError).toBe(true);
        },
      ),
      { numRuns: 100 },
    );
  });
});
