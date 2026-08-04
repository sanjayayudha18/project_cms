import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { PageHeader } from '../PageHeader';

describe('PageHeader', () => {
  it('renders eyebrow, title, and description', () => {
    render(
      <PageHeader
        eyebrow="Cash operations"
        title="Replenishment schedules"
        description="Plan and monitor vendor routes."
      />
    );

    expect(screen.getByText('Cash operations')).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 1, name: 'Replenishment schedules' })).toBeInTheDocument();
    expect(screen.getByText('Plan and monitor vendor routes.')).toBeInTheDocument();
  });

  it('does not render description when not provided', () => {
    render(
      <PageHeader eyebrow="Financial control" title="Reconciliation" />
    );

    expect(screen.getByText('Financial control')).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 1, name: 'Reconciliation' })).toBeInTheDocument();
    // No <p> element should be present
    expect(screen.queryByRole('paragraph')).not.toBeInTheDocument();
  });

  it('renders actions slot when provided', () => {
    render(
      <PageHeader
        eyebrow="Cash operations"
        title="Replenishment schedules"
        actions={<button>New schedule</button>}
      />
    );

    expect(screen.getByRole('button', { name: 'New schedule' })).toBeInTheDocument();
  });

  it('does not render actions container when no actions provided', () => {
    const { container } = render(
      <PageHeader eyebrow="Test" title="Test Title" />
    );

    // The actions wrapper div should not exist
    const header = container.querySelector('header');
    const actionsDivs = header?.querySelectorAll('.flex.items-center.gap-3');
    expect(actionsDivs?.length ?? 0).toBe(0);
  });

  it('applies correct eyebrow styling classes', () => {
    render(<PageHeader eyebrow="Cash operations" title="Test" />);

    const eyebrow = screen.getByText('Cash operations');
    expect(eyebrow).toHaveClass('uppercase');
    expect(eyebrow.tagName).toBe('SPAN');
  });

  it('renders title as h1 element', () => {
    render(<PageHeader eyebrow="Test" title="Page Title" />);

    const heading = screen.getByRole('heading', { level: 1 });
    expect(heading).toHaveTextContent('Page Title');
  });
});
