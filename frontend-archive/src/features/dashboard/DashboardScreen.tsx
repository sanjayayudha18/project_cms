import { PageHeader } from '@/components/ui/PageHeader';
import { Button } from '@/components/ui/Button';
import { formatFullDate, getGreeting } from '@/lib/formatters';
import { useToast } from '@/context/ToastContext';
import { MetricStrip } from './MetricStrip';
import { ReplenishmentSummary } from './ReplenishmentSummary';
import { AttentionPanel } from './AttentionPanel';

/**
 * Dashboard landing screen — operational overview.
 * Renders greeting header, KPI metric strip, replenishment summary table,
 * and attention panel in a responsive grid layout.
 *
 * @validates Requirements 3.1, 3.2, 4.1, 5.5, 8.5, 9.4
 */
export function DashboardScreen() {
  const { showToast } = useToast();
  const now = new Date();
  const greeting = getGreeting(now.getHours());
  const dateStr = formatFullDate(now);

  return (
    <div className="py-6 max-[759px]:py-4">
      <PageHeader
        eyebrow={dateStr}
        title={`${greeting}, Raden`}
        description="Here's what's happening across your cash operations today."
        actions={
          <>
            <Button variant="secondary" className="h-11 px-5">
              Export DSR
            </Button>
            <Button
              variant="primary"
              className="h-11 px-5"
              onClick={() => showToast('Schedule created successfully', 'success')}
            >
              New schedule
            </Button>
          </>
        }
      />

      <MetricStrip />

      <div className="grid grid-cols-1 min-[1080px]:grid-cols-[minmax(0,1.55fr)_minmax(300px,0.75fr)] gap-7 mt-8">
        <ReplenishmentSummary />
        <AttentionPanel />
      </div>
    </div>
  );
}
