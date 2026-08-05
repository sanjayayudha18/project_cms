import * as fc from 'fast-check';

/**
 * Property 10: Schedule Date Grouping Aggregation
 * Validates: Requirements 6.5
 *
 * For any set of replenishment schedules, grouping by scheduledDate should produce groups where:
 * - Each group's computed total equals the sum of recommendedAmount for all schedules in that date
 * - Each group's count equals the number of schedules for that date
 * - The union of all groups equals the original set (no records lost or duplicated)
 *
 * Property 11: Schedule Multi-Level Sort
 * Validates: Requirements 6.6
 *
 * For any set of replenishment schedules, after applying the default sort:
 * - Dates across groups should be in strictly non-decreasing order (ascending)
 * - Within each date group, priorities should be in non-increasing order (High > Medium > Low)
 * - The sorted output should be a permutation of the input
 */

// Types matching the production code
type Priority = 'High' | 'Medium' | 'Low';
type ScheduleStatus = 'Pending' | 'Confirmed' | 'Executed' | 'Cancelled';

interface Schedule {
  id: string;
  scheduledDate: string;
  recommendedAmount: number;
  priority: Priority;
  status: ScheduleStatus;
}

const priorityOrder: Record<Priority, number> = { High: 3, Medium: 2, Low: 1 };

interface DateGroup {
  date: string;
  totalAmount: number;
  count: number;
  schedules: Schedule[];
}

// Replicate the pure groupAndSort logic from SchedulePage.tsx for testing
function groupAndSort(schedules: Schedule[]): DateGroup[] {
  const groupMap = new Map<string, Schedule[]>();

  for (const s of schedules) {
    const existing = groupMap.get(s.scheduledDate);
    if (existing) {
      existing.push(s);
    } else {
      groupMap.set(s.scheduledDate, [s]);
    }
  }

  const sortedDates = [...groupMap.keys()].sort((a, b) => a.localeCompare(b));

  return sortedDates.map((date) => {
    const items = groupMap.get(date)!;
    items.sort((a, b) => priorityOrder[b.priority] - priorityOrder[a.priority]);
    const totalAmount = items.reduce((sum, s) => sum + s.recommendedAmount, 0);
    return { date, totalAmount, count: items.length, schedules: items };
  });
}

// --- Generators ---

const priorityArb = fc.constantFrom<Priority>('High', 'Medium', 'Low');
const statusArb = fc.constantFrom<ScheduleStatus>('Pending', 'Confirmed', 'Executed', 'Cancelled');
const dateArb = fc.integer({ min: 1, max: 30 }).map(
  (d) => `2024-01-${String(d).padStart(2, '0')}`,
);

const scheduleArb = fc.record({
  id: fc.string({ minLength: 1, maxLength: 10 }),
  scheduledDate: dateArb,
  recommendedAmount: fc.integer({ min: 25_000_000, max: 500_000_000 }),
  priority: priorityArb,
  status: statusArb,
});

const schedulesArb = fc.array(scheduleArb, { minLength: 0, maxLength: 50 });

// --- Property 10: Schedule Date Grouping Aggregation ---

describe('Property 10: Schedule Date Grouping Aggregation', () => {
  it('each group totalAmount equals the sum of recommendedAmount for schedules in that group', () => {
    fc.assert(
      fc.property(schedulesArb, (schedules) => {
        const groups = groupAndSort(schedules);

        for (const group of groups) {
          const expectedTotal = group.schedules.reduce(
            (sum, s) => sum + s.recommendedAmount,
            0,
          );
          expect(group.totalAmount).toBe(expectedTotal);
        }
      }),
      { numRuns: 100 },
    );
  });

  it('each group count equals the number of schedules for that date', () => {
    fc.assert(
      fc.property(schedulesArb, (schedules) => {
        const groups = groupAndSort(schedules);

        for (const group of groups) {
          expect(group.count).toBe(group.schedules.length);
        }
      }),
      { numRuns: 100 },
    );
  });

  it('union of all groups equals the original set (no records lost or duplicated)', () => {
    fc.assert(
      fc.property(schedulesArb, (schedules) => {
        const groups = groupAndSort(schedules);

        // Flatten all schedules from all groups
        const allFromGroups = groups.flatMap((g) => g.schedules);

        // Same total count
        expect(allFromGroups.length).toBe(schedules.length);

        // Every schedule from the input appears in exactly one group
        const inputIds = schedules.map((s) => s.id).sort();
        const outputIds = allFromGroups.map((s) => s.id).sort();
        expect(outputIds).toEqual(inputIds);

        // Also verify amounts match (stronger permutation check)
        const inputAmounts = schedules
          .map((s) => s.recommendedAmount)
          .sort((a, b) => a - b);
        const outputAmounts = allFromGroups
          .map((s) => s.recommendedAmount)
          .sort((a, b) => a - b);
        expect(outputAmounts).toEqual(inputAmounts);
      }),
      { numRuns: 100 },
    );
  });
});

// --- Property 11: Schedule Multi-Level Sort ---

describe('Property 11: Schedule Multi-Level Sort', () => {
  it('dates across groups are in strictly non-decreasing order (ascending)', () => {
    fc.assert(
      fc.property(schedulesArb, (schedules) => {
        const groups = groupAndSort(schedules);

        for (let i = 1; i < groups.length; i++) {
          expect(groups[i - 1].date <= groups[i].date).toBe(true);
        }
      }),
      { numRuns: 100 },
    );
  });

  it('within each group, priorities are in non-increasing order (High >= Medium >= Low)', () => {
    fc.assert(
      fc.property(schedulesArb, (schedules) => {
        const groups = groupAndSort(schedules);

        for (const group of groups) {
          for (let i = 1; i < group.schedules.length; i++) {
            const prevPriority = priorityOrder[group.schedules[i - 1].priority];
            const currPriority = priorityOrder[group.schedules[i].priority];
            expect(prevPriority >= currPriority).toBe(true);
          }
        }
      }),
      { numRuns: 100 },
    );
  });

  it('sorted output is a permutation of the input', () => {
    fc.assert(
      fc.property(schedulesArb, (schedules) => {
        const groups = groupAndSort(schedules);
        const allFromGroups = groups.flatMap((g) => g.schedules);

        // Same length
        expect(allFromGroups.length).toBe(schedules.length);

        // Same set of elements (check by sorting both on multiple fields)
        const serialize = (s: Schedule) =>
          `${s.id}|${s.scheduledDate}|${s.recommendedAmount}|${s.priority}|${s.status}`;

        const inputSerialized = schedules.map(serialize).sort();
        const outputSerialized = allFromGroups.map(serialize).sort();
        expect(outputSerialized).toEqual(inputSerialized);
      }),
      { numRuns: 100 },
    );
  });
});
