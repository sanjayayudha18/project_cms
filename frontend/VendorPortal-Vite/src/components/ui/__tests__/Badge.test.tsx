import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { Badge } from '@/components/ui/Badge';
import { Star } from 'lucide-react';

describe('Badge', () => {
  it('renders with info variant and correct classes', () => {
    render(<Badge variant="info">Scheduled</Badge>);
    const badge = screen.getByText('Scheduled').closest('span[class*="bg-info"]');
    expect(badge).toBeInTheDocument();
    expect(badge).toHaveClass('bg-info-bg', 'text-info-fg');
  });

  it('renders with warning variant and correct classes', () => {
    render(<Badge variant="warning">In Transit</Badge>);
    const badge = screen.getByText('In Transit').closest('span[class*="bg-warning"]');
    expect(badge).toBeInTheDocument();
    expect(badge).toHaveClass('bg-warning-bg', 'text-warning-fg');
  });

  it('renders with success variant and correct classes', () => {
    render(<Badge variant="success">Completed</Badge>);
    const badge = screen.getByText('Completed').closest('span[class*="bg-success"]');
    expect(badge).toBeInTheDocument();
    expect(badge).toHaveClass('bg-success-bg', 'text-success-fg');
  });

  it('renders with danger variant and correct classes', () => {
    render(<Badge variant="danger">Failed</Badge>);
    const badge = screen.getByText('Failed').closest('span[class*="bg-danger"]');
    expect(badge).toBeInTheDocument();
    expect(badge).toHaveClass('bg-danger-bg', 'text-danger-fg');
  });

  it('renders with neutral variant and correct classes', () => {
    render(<Badge variant="neutral">Manual</Badge>);
    const badge = screen.getByText('Manual').closest('span[class*="bg-neutral"]');
    expect(badge).toBeInTheDocument();
    expect(badge).toHaveClass('bg-neutral-100', 'text-neutral-600');
  });

  it('renders icon alongside text label (never color alone)', () => {
    const { container } = render(<Badge variant="info">Scheduled</Badge>);
    // Icon (svg) should be present
    const svg = container.querySelector('svg');
    expect(svg).toBeInTheDocument();
    // Text label should be present
    expect(screen.getByText('Scheduled')).toBeInTheDocument();
  });

  it('renders custom icon when provided', () => {
    const { container } = render(
      <Badge variant="success" icon={Star}>
        Starred
      </Badge>,
    );
    // The custom icon (Star) should render — verify SVG is present
    const svg = container.querySelector('svg');
    expect(svg).toBeInTheDocument();
    expect(screen.getByText('Starred')).toBeInTheDocument();
  });

  it('renders default icon when no custom icon is provided', () => {
    const { container } = render(<Badge variant="danger">Error</Badge>);
    // Default danger icon is XCircle — an SVG should be rendered
    const svg = container.querySelector('svg');
    expect(svg).toBeInTheDocument();
    expect(svg).toHaveAttribute('aria-hidden', 'true');
  });
});
