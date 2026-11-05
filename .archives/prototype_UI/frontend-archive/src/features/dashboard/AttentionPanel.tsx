import {
  TriangleAlert,
  AlertCircle,
  Clock,
  Truck,
  FileCheck,
  type LucideIcon,
} from 'lucide-react';
import attentionData from '@/data/attention-items.json';

/* ─── Types ─── */

interface AttentionItem {
  id: string;
  category: 'danger' | 'warning' | 'info';
  icon: string;
  title: string;
  description: string;
  time: string;
}

/* ─── Icon mapping ─── */

const iconMap: Record<string, LucideIcon> = {
  'triangle-alert': TriangleAlert,
  'alert-circle': AlertCircle,
  clock: Clock,
  truck: Truck,
  'file-check': FileCheck,
};

/* ─── Semantic color classes for icon container ─── */

const categoryStyles: Record<
  AttentionItem['category'],
  { bg: string; text: string }
> = {
  danger: {
    bg: 'bg-[var(--danger-bg)]',
    text: 'text-[var(--danger-fg)]',
  },
  warning: {
    bg: 'bg-[var(--warning-bg)]',
    text: 'text-[var(--warning-fg)]',
  },
  info: {
    bg: 'bg-[var(--info-bg)]',
    text: 'text-[var(--info-fg)]',
  },
};

/* ─── Component ─── */

export function AttentionPanel() {
  const items = attentionData as AttentionItem[];

  return (
    <aside
      className="rounded-[var(--radius-lg)] border border-[var(--n-200)] bg-[var(--n-0)] p-5"
      aria-label="Needs attention"
    >
      {/* Header */}
      <div className="flex items-center gap-3 mb-4">
        <h2 className="text-[var(--text-lg)] font-semibold text-[var(--n-900)] leading-tight">
          Needs attention
        </h2>
        <span
          className="inline-flex items-center justify-center w-6 h-6 rounded-full bg-[var(--n-100)] text-[var(--n-700)] text-xs font-medium"
          aria-label={`${items.length} items`}
        >
          {items.length}
        </span>
      </div>

      {/* Items list */}
      <ul className="flex flex-col divide-y divide-[var(--n-100)]">
        {items.map((item) => (
          <AttentionListItem key={item.id} item={item} />
        ))}
      </ul>
    </aside>
  );
}

/* ─── List Item ─── */

function AttentionListItem({ item }: { item: AttentionItem }) {
  const Icon = iconMap[item.icon] ?? AlertCircle;
  const style = categoryStyles[item.category];

  return (
    <li className="flex items-start gap-3 py-3 first:pt-0 last:pb-0">
      {/* Icon container — 36×36px rounded square */}
      <div
        className={`flex-shrink-0 flex items-center justify-center w-9 h-9 rounded-[var(--radius-md)] ${style.bg} ${style.text}`}
      >
        <Icon size={18} strokeWidth={2} aria-hidden="true" />
      </div>

      {/* Text content */}
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium text-[var(--n-900)] leading-snug truncate">
          {item.title}
        </p>
        <p className="text-xs text-[var(--n-500)] leading-relaxed line-clamp-2 mt-0.5">
          {item.description}
        </p>
      </div>

      {/* Relative timestamp */}
      <span className="flex-shrink-0 text-xs text-[var(--n-400)] mt-0.5">
        {item.time}
      </span>
    </li>
  );
}
