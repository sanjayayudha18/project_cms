# Design Document: Cash Flow Monitoring

## Overview

The Cash Flow Monitoring feature is a **read-only dashboard page** rendered at `/cash-flow`. It provides a consolidated view of daily cash flow per vendor, KPI stats cards, and ATM cash level indicators. The page follows the existing feature-module pattern (`src/features/cash-flow/`) and composes shared UI components (`PageHeader`, `Badge`, `Card`) with feature-specific components (`StatsCard`, `VendorBarChart`, `AtmLevelTable`).

Data flows through a single TanStack Query hook (`useCashFlowData`) that returns mock data during development and will later proxy to the Go backend API endpoint (`GET /api/v1/cash-flow/summary`).

---

## Architecture

### Data Flow

```
Go Backend (GET /api/v1/cash-flow/summary)
    ↓ JSON response
TanStack Query (useCashFlowData hook, 5-min staleTime)
    ↓ CashFlowSummary
CashFlowScreen (composed layout)
    ├── StatsCardGrid (4 KPI cards)
    ├── VendorBarChartPanel (Recharts grouped bar chart)
    └── AtmLevelTablePanel (progress bar table)
```

### Component Tree

```
AppShell
└── CashFlowScreen
    ├── PageHeader (title, description)
    │   └── Badge (DataSourceBadge — variant="info")
    ├── StatsCardGrid
    │   ├── StatsCard × 4
    │   │   └── TrendIndicator (optional)
    └── SplitLayout
        ├── VendorBarChartPanel (left)
        │   ├── PanelHeader (title + period badge)
        │   └── VendorBarChart (Recharts BarChart)
        │       └── Legend
        └── AtmLevelTablePanel (right)
            ├── PanelHeader (title)
            └── AtmLevelTable
                └── AtmLevelRow × N
                    ├── ATM identifier (monospace)
                    ├── ProgressBar (semantic color)
                    └── Percentage value (tabular-nums)
```

### File Structure

```
src/features/cash-flow/
├── index.ts                  # Barrel export
├── CashFlowScreen.tsx        # Page component (composed layout)
├── StatsCard.tsx             # Individual metric card with trend
├── StatsCardGrid.tsx         # 4-card responsive grid wrapper
├── VendorBarChart.tsx        # Recharts grouped bar chart
├── AtmLevelTable.tsx         # Table with progress bars
├── useCashFlowData.ts        # TanStack Query hook
├── types.ts                  # TypeScript interfaces
├── constants.ts              # Vendor colors, chart config
└── __tests__/
    ├── CashFlowScreen.test.tsx
    ├── StatsCard.test.tsx
    ├── VendorBarChart.test.tsx
    ├── AtmLevelTable.test.tsx
    ├── useCashFlowData.test.ts
    └── cash-flow.property.test.ts
```

### Navigation Integration

Add a third group to `NAV_GROUPS` in `src/lib/constants.ts`:

```typescript
import { Activity } from 'lucide-react';

// Appended after existing groups:
{
  label: 'Monitoring',
  items: [
    { path: '/cash-flow', label: 'Cash Flow Monitoring', icon: Activity },
  ],
}
```

Route added to `src/app/routes.tsx`:

```typescript
import { CashFlowScreen } from '@/features/cash-flow';

{ path: '/cash-flow', element: <CashFlowScreen /> },
```

---

## Components and Interfaces

### Hook Signature & Query Key Convention

```typescript
// src/features/cash-flow/useCashFlowData.ts

import { useQuery } from '@tanstack/react-query';
import type { CashFlowSummary, UseCashFlowDataReturn } from './types';

/**
 * Query key follows the project convention: [domain, scope].
 * Future variations: ['cash-flow', 'summary', { date: '2026-07-21' }]
 */
const CASH_FLOW_QUERY_KEY = ['cash-flow', 'summary'] as const;

export function useCashFlowData(): UseCashFlowDataReturn {
  const query = useQuery<CashFlowSummary>({
    queryKey: CASH_FLOW_QUERY_KEY,
    queryFn: fetchCashFlowSummary,
    staleTime: 5 * 60 * 1000, // 5 min — EOD data doesn't change frequently
  });

  return {
    data: query.data,
    isLoading: query.isLoading,
    isError: query.isError,
    error: query.error,
    refetch: query.refetch,
  };
}

/**
 * Mock fetch function — returns static data matching the ClickUp prototype.
 * Will be replaced with: GET /api/v1/cash-flow/summary
 */
async function fetchCashFlowSummary(): Promise<CashFlowSummary> {
  // Simulated network delay
  await new Promise((resolve) => setTimeout(resolve, 300));
  return MOCK_CASH_FLOW_DATA;
}
```

### VendorBarChart (Recharts Configuration)

```typescript
// src/features/cash-flow/VendorBarChart.tsx

import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts';
import type { VendorDayFlow, VendorConfig } from './types';

interface VendorBarChartProps {
  readonly data: readonly VendorDayFlow[];
  readonly vendors: readonly VendorConfig[];
}

export function VendorBarChart({ data, vendors }: VendorBarChartProps) {
  return (
    <div
      className="min-h-[240px] w-full"
      aria-label="Bar chart showing daily cash flow per vendor for the past 7 days"
    >
      <ResponsiveContainer width="100%" height={280}>
        <BarChart data={[...data]} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
          <CartesianGrid
            strokeDasharray="3 3"
            stroke="oklch(0.908 0.006 29)" /* --n-200 */
            vertical={false}
          />
          <XAxis
            dataKey="date"
            tick={{ fontSize: 12, fill: 'oklch(0.560 0.009 29)' /* --n-500 */ }}
            tickFormatter={formatShortDate}
          />
          <YAxis
            tick={{ fontSize: 12, fill: 'oklch(0.560 0.009 29)' }}
            tickFormatter={formatChartValue}
          />
          <Tooltip content={<CustomTooltip />} />
          <Legend
            verticalAlign="bottom"
            wrapperStyle={{ paddingTop: 12 }}
          />
          {vendors.map((vendor) => (
            <Bar
              key={vendor.name}
              dataKey={vendor.name}
              fill={vendor.color}
              radius={[3, 3, 0, 0]}
              maxBarSize={32}
            />
          ))}
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
```

**Vendor color palette** — distinct OKLCH hues chosen for AA contrast against `--n-0` background:

```typescript
// src/features/cash-flow/constants.ts

import type { VendorConfig } from './types';

export const VENDOR_COLORS: readonly VendorConfig[] = [
  { name: 'Abacus', color: 'oklch(0.55 0.18 245)' },       // blue
  { name: 'Bijak Jakarta', color: 'oklch(0.60 0.16 155)' }, // green
  { name: 'Advantage', color: 'oklch(0.58 0.17 29)' },      // red (brand)
  { name: 'Indoguard', color: 'oklch(0.55 0.14 300)' },     // purple
  { name: 'SSI', color: 'oklch(0.60 0.15 78)' },            // amber
];
```

### StatsCard Component

```typescript
// src/features/cash-flow/StatsCard.tsx

import type { StatsCardData, TrendDirection } from './types';

interface StatsCardProps {
  readonly data: StatsCardData;
}

export function StatsCard({ data }: StatsCardProps) {
  const { label, icon: Icon, value, subtitle, trend } = data;

  return (
    <div className="p-5">
      <div className="flex items-center gap-1.5 text-xs text-[var(--n-500)]">
        <Icon size={14} aria-hidden="true" />
        <span>{label}</span>
      </div>
      <div className="mt-1 text-2xl font-bold tabular-nums text-[var(--n-900)]">
        {value}
      </div>
      <div className="mt-1 flex items-center gap-2 text-xs">
        {subtitle && <span className="text-[var(--n-500)]">{subtitle}</span>}
        {trend && <TrendIndicator direction={trend.direction} percentage={trend.percentage} />}
      </div>
    </div>
  );
}

function TrendIndicator({ direction, percentage }: { direction: TrendDirection; percentage: number }) {
  const isUp = direction === 'up';
  const colorClass = isUp ? 'text-success-fg' : 'text-danger-fg';
  const arrow = isUp ? '↑' : '↓';
  const srLabel = isUp ? 'Increased' : 'Decreased';

  return (
    <span className={`inline-flex items-center gap-0.5 ${colorClass}`}>
      <span aria-hidden="true">{arrow}</span>
      <span className="sr-only">{srLabel}</span>
      <span>{percentage}%</span>
    </span>
  );
}
```

### ATM Level Progress Bar — Semantic Color Logic

```typescript
// src/features/cash-flow/AtmLevelTable.tsx

import type { AtmLevel, CashLevelTier } from './types';

/**
 * Determine the semantic color tier based on ATM cash level percentage.
 *
 * - >= 50%: success (green)
 * - 20–49%: warning (amber)
 * - < 20%: danger (red)
 */
export function getCashLevelTier(percentage: number): CashLevelTier {
  if (percentage >= 50) return 'success';
  if (percentage >= 20) return 'warning';
  return 'danger';
}

const tierStyles: Record<CashLevelTier, string> = {
  success: 'bg-[oklch(0.560_0.130_155)]', // --success-solid
  warning: 'bg-[oklch(0.760_0.150_78)]',  // --warning-solid
  danger: 'bg-[oklch(0.545_0.205_12)]',   // --danger-solid
};

export function AtmLevelRow({ atm }: { atm: AtmLevel }) {
  const tier = getCashLevelTier(atm.percentage);

  return (
    <tr>
      <td className="font-mono text-sm text-[var(--n-800)] py-2">{atm.label}</td>
      <td className="py-2 px-3 w-full">
        <div
          className="h-2 rounded-full bg-[var(--n-100)] overflow-hidden"
          role="progressbar"
          aria-valuenow={atm.percentage}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-label={`${atm.label} cash level ${atm.percentage}%`}
        >
          <div
            className={`h-full rounded-full transition-all ${tierStyles[tier]}`}
            style={{ width: `${atm.percentage}%` }}
          />
        </div>
      </td>
      <td className="text-right text-sm tabular-nums text-[var(--n-700)] py-2 whitespace-nowrap">
        {atm.percentage}%
      </td>
    </tr>
  );
}
```

### CashFlowScreen (Page Layout)

```typescript
// CashFlowScreen.tsx — layout skeleton

export function CashFlowScreen() {
  const { data, isLoading, isError, refetch } = useCashFlowData();

  if (isLoading) return <CashFlowSkeleton />;
  if (isError) return <ErrorState onRetry={refetch} />;

  return (
    <div className="py-6 max-[759px]:py-4">
      {/* Page header */}
      <PageHeader
        eyebrow="monitoring"
        title="Cash Flow Monitoring"
        description="Baca dari read replica · basis summary EOD per 00:00 · periode Jul 2026"
      />
      <Badge variant="info" icon={Database} label="Sumber: EOD H-1" />

      {/* KPI Stats Cards */}
      <StatsCardGrid stats={data.stats} />

      {/* Split layout: chart + table */}
      <div className="grid grid-cols-1 min-[1024px]:grid-cols-[1.5fr_1fr] gap-6 mt-8">
        <VendorBarChartPanel
          data={data.vendorChart.data}
          vendors={data.vendorChart.vendors}
        />
        <AtmLevelTablePanel levels={data.atmLevels} />
      </div>
    </div>
  );
}
```

### Responsive Breakpoint Strategy

| Breakpoint | Layout Behavior |
|---|---|
| `min-[1024px]` | Split layout: 2-column grid `grid-cols-[1.5fr_1fr]` |
| `< 1024px` | Single column, chart above table |
| `min-[768px]` | Stats cards: 4-column grid |
| `< 768px` & `min-[480px]` | Stats cards: 2-column grid |
| `< 480px` | Stats cards: single column |

### Heading Hierarchy

| Level | Element | Content |
|---|---|---|
| `h1` | `PageHeader` | "Cash Flow Monitoring" |
| `h2` | `VendorBarChartPanel` header | "Cash Flow Harian per Vendor" |
| `h2` | `AtmLevelTablePanel` header | "Level Kas per ATM" |

---

## Data Models

```typescript
// src/features/cash-flow/types.ts

import type { LucideIcon } from 'lucide-react';

/** Direction of a trend indicator */
export type TrendDirection = 'up' | 'down';

/** Semantic color tier for ATM cash levels */
export type CashLevelTier = 'success' | 'warning' | 'danger';

/** A single KPI stats card data */
export interface StatsCardData {
  readonly label: string;
  readonly icon: LucideIcon;
  readonly value: string;
  readonly subtitle?: string;
  readonly trend?: {
    readonly direction: TrendDirection;
    readonly percentage: number;
  };
}

/** A single day's cash flow per vendor (chart data point) */
export interface VendorDayFlow {
  readonly date: string; // ISO date string (YYYY-MM-DD)
  readonly [vendorName: string]: number | string; // vendor amounts + date key
}

/** Vendor metadata for chart legend and color mapping */
export interface VendorConfig {
  readonly name: string;
  readonly color: string; // OKLCH color value
}

/** A single ATM cash level row */
export interface AtmLevel {
  readonly id: string;       // e.g. "ATM-00417"
  readonly label: string;    // Display identifier
  readonly percentage: number; // 0–100
}

/** Complete data structure returned by useCashFlowData */
export interface CashFlowSummary {
  readonly stats: readonly StatsCardData[];
  readonly vendorChart: {
    readonly data: readonly VendorDayFlow[];
    readonly vendors: readonly VendorConfig[];
  };
  readonly atmLevels: readonly AtmLevel[];
}

/** Hook return type wrapping TanStack Query state */
export interface UseCashFlowDataReturn {
  readonly data: CashFlowSummary | undefined;
  readonly isLoading: boolean;
  readonly isError: boolean;
  readonly error: Error | null;
  readonly refetch: () => void;
}
```

---

## Error Handling

### Loading State

A skeleton placeholder (`CashFlowSkeleton`) renders pulse-animated blocks matching the page layout while data is being fetched:

```typescript
function CashFlowSkeleton() {
  return (
    <div className="py-6 animate-pulse space-y-6">
      <div className="h-8 w-64 bg-n-200 rounded" />
      <div className="grid grid-cols-1 min-[768px]:grid-cols-4 gap-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="h-24 bg-n-100 rounded-lg" />
        ))}
      </div>
      <div className="grid grid-cols-1 min-[1024px]:grid-cols-[1.5fr_1fr] gap-6">
        <div className="h-72 bg-n-100 rounded-lg" />
        <div className="h-72 bg-n-100 rounded-lg" />
      </div>
    </div>
  );
}
```

### Error State

When the query fails, a danger-colored banner with a retry button is shown:

```typescript
function ErrorState({ onRetry }: { onRetry: () => void }) {
  return (
    <div className="py-6">
      <div className="bg-danger-bg text-danger-fg rounded-lg p-4 text-sm flex items-center justify-between">
        <span>Gagal memuat data cash flow. Silakan coba lagi.</span>
        <button
          onClick={onRetry}
          className="text-sm font-medium underline hover:no-underline"
        >
          Retry
        </button>
      </div>
    </div>
  );
}
```

### Query Error Recovery

TanStack Query handles retry logic automatically (3 retries with exponential backoff by default). The `refetch` function is exposed for manual retry via the error state UI.

---

## Testing Strategy

### Unit Tests

| Test File | Covers |
|---|---|
| `StatsCard.test.tsx` | Renders label, value, subtitle, trend indicator with correct a11y |
| `VendorBarChart.test.tsx` | Renders correct number of bars, legend items, aria-label |
| `AtmLevelTable.test.tsx` | Semantic color tiers, progressbar ARIA attributes |
| `useCashFlowData.test.ts` | Query key, loading/error/success states |

### Property-Based Tests

| Property | Assertion |
|---|---|
| Trend indicator accessibility | Non-null trend always renders arrow + sr-only label + correct color class |
| Vendor chart color contrast | All vendor colors pass 3:1 contrast ratio against `--n-0` |
| ATM level semantic mapping | `getCashLevelTier` returns correct tier for all values in [0, 100] |
| ARIA progressbar completeness | Every `AtmLevelRow` has `role`, `aria-valuenow`, `aria-valuemin`, `aria-valuemax` |

### Integration Test

`CashFlowScreen.test.tsx` — mounts the full page with mocked query, verifies:
- All 4 stats cards render
- Chart container renders with correct aria-label
- ATM table rows match mock data count
- Loading skeleton shows during fetch
- Error state renders on query failure with retry button

### E2E (Playwright)

Covered by `e2e/cash-flow.spec.ts`:
- Navigation to `/cash-flow` from sidebar
- Page renders without console errors
- Responsive layout shift at 1024px breakpoint

---

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system — essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Trend indicator accessibility and correctness

*For any* `StatsCardData` with a non-null `trend` field, the rendered `TrendIndicator` SHALL include both a text-based direction label (arrow character or screen-reader-only text) AND the correct semantic color class (`text-success-fg` for `'up'`, `text-danger-fg` for `'down'`).

**Validates: Requirements 3.6, 9.3**

### Property 2: Vendor chart color contrast

*For any* pair of vendor colors defined in `VENDOR_COLORS`, the WCAG 2.1 AA contrast ratio against the chart background (`--n-0`: `oklch(0.992 0.003 29)`) SHALL be at least 3:1 for graphical objects.

**Validates: Requirements 4.4**

### Property 3: ATM level semantic color mapping

*For any* `AtmLevel` entry where `percentage` is in the range [0, 100], calling `getCashLevelTier(percentage)` SHALL return `'success'` when percentage ≥ 50, `'warning'` when 20 ≤ percentage < 50, and `'danger'` when percentage < 20.

**Validates: Requirements 5.5**

### Property 4: ARIA progressbar attribute completeness

*For any* rendered `AtmLevelRow`, the progress bar element SHALL have `role="progressbar"` with `aria-valuenow` equal to the ATM's percentage, `aria-valuemin` equal to 0, and `aria-valuemax` equal to 100.

**Validates: Requirements 9.2**
