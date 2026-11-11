/**
 * Unit tests for formatter utilities.
 * @validates Requirements 1.6, 2.5, 3.1, 3.3, 4.3, 7.5, 11.6
 */
import { describe, it, expect } from 'vitest';
import {
  formatIDR,
  formatIDRFull,
  getGreeting,
  formatFullDate,
  progressPercent,
  formatBadgeCount,
  getInitials,
  formatDifference,
} from '../formatters';

describe('formatIDR', () => {
  it('formats trillions with T suffix', () => {
    expect(formatIDR(18_420_000_000_000)).toBe('IDR 18.4T');
    expect(formatIDR(1_000_000_000_000)).toBe('IDR 1.0T');
  });

  it('formats billions with B suffix', () => {
    expect(formatIDR(12_800_000_000)).toBe('IDR 12.8B');
    expect(formatIDR(1_000_000_000)).toBe('IDR 1.0B');
  });

  it('formats millions with M suffix', () => {
    expect(formatIDR(125_000_000)).toBe('IDR 125.0M');
    expect(formatIDR(1_000_000)).toBe('IDR 1.0M');
  });

  it('formats values below 1 million as raw numbers', () => {
    expect(formatIDR(50_000)).toBe('IDR 50000');
    expect(formatIDR(0)).toBe('IDR 0');
  });
});

describe('formatIDRFull', () => {
  it('formats with dot-separated thousands', () => {
    expect(formatIDRFull(12_800_000_000)).toBe('IDR 12.800.000.000');
    expect(formatIDRFull(125_000_000)).toBe('IDR 125.000.000');
  });

  it('formats zero', () => {
    expect(formatIDRFull(0)).toBe('IDR 0');
  });

  it('formats small values', () => {
    expect(formatIDRFull(1_000)).toBe('IDR 1.000');
  });
});

describe('getGreeting', () => {
  it('returns "Good morning" for hours 0-11', () => {
    expect(getGreeting(0)).toBe('Good morning');
    expect(getGreeting(6)).toBe('Good morning');
    expect(getGreeting(11)).toBe('Good morning');
  });

  it('returns "Good afternoon" for hours 12-17', () => {
    expect(getGreeting(12)).toBe('Good afternoon');
    expect(getGreeting(14)).toBe('Good afternoon');
    expect(getGreeting(17)).toBe('Good afternoon');
  });

  it('returns "Good evening" for hours 18-23', () => {
    expect(getGreeting(18)).toBe('Good evening');
    expect(getGreeting(20)).toBe('Good evening');
    expect(getGreeting(23)).toBe('Good evening');
  });
});

describe('formatFullDate', () => {
  it('formats date as "Weekday, Day Month Year"', () => {
    const date = new Date('2026-07-21T00:00:00');
    const result = formatFullDate(date);
    expect(result).toBe('Tuesday, 21 July 2026');
  });
});

describe('progressPercent', () => {
  it('calculates percentage correctly', () => {
    expect(progressPercent(15, 18)).toBe(83);
    expect(progressPercent(18, 18)).toBe(100);
    expect(progressPercent(0, 18)).toBe(0);
  });

  it('returns 0 when total is 0', () => {
    expect(progressPercent(0, 0)).toBe(0);
  });

  it('rounds to nearest integer', () => {
    expect(progressPercent(1, 3)).toBe(33);
    expect(progressPercent(2, 3)).toBe(67);
  });
});

describe('formatBadgeCount', () => {
  it('returns null for 0', () => {
    expect(formatBadgeCount(0)).toBeNull();
  });

  it('returns null for negative values', () => {
    expect(formatBadgeCount(-1)).toBeNull();
    expect(formatBadgeCount(-100)).toBeNull();
  });

  it('returns count as string for 1-99', () => {
    expect(formatBadgeCount(1)).toBe('1');
    expect(formatBadgeCount(50)).toBe('50');
    expect(formatBadgeCount(99)).toBe('99');
  });

  it('returns "99+" for values above 99', () => {
    expect(formatBadgeCount(100)).toBe('99+');
    expect(formatBadgeCount(999)).toBe('99+');
  });
});

describe('getInitials', () => {
  it('returns first + last initials for two-word names', () => {
    expect(getInitials('Raden Budiman')).toBe('RB');
  });

  it('returns single initial for single name', () => {
    expect(getInitials('Raden')).toBe('R');
  });

  it('returns empty string for empty input', () => {
    expect(getInitials('')).toBe('');
  });

  it('handles multiple names (uses first and last)', () => {
    expect(getInitials('Raden Budi Yudha')).toBe('RY');
  });

  it('handles leading/trailing whitespace', () => {
    expect(getInitials('  Raden Budiman  ')).toBe('RB');
  });

  it('uppercases lowercase initials', () => {
    expect(getInitials('raden budiman')).toBe('RB');
  });
});

describe('formatDifference', () => {
  it('formats negative values with minus prefix and danger class', () => {
    expect(formatDifference(-125_000_000)).toEqual({
      text: '- IDR 125.000.000',
      colorClass: 'text-danger',
    });
  });

  it('formats positive values with plus prefix and success class', () => {
    expect(formatDifference(50_000_000)).toEqual({
      text: '+ IDR 50.000.000',
      colorClass: 'text-success',
    });
  });

  it('formats zero with no sign and no color class', () => {
    expect(formatDifference(0)).toEqual({
      text: 'IDR 0',
      colorClass: '',
    });
  });
});
