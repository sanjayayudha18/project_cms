import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect } from 'vitest';

import { ToastProvider } from '@/context/ToastContext';
import { ReconciliationScreen } from '../ReconciliationScreen';

function renderScreen() {
  return render(
    <MemoryRouter>
      <ToastProvider>
        <ReconciliationScreen />
      </ToastProvider>
    </MemoryRouter>,
  );
}

/** Find a select element by its sibling label text */
function getSelectByLabel(labelText: string): HTMLSelectElement {
  const label = screen.getByText(labelText, { selector: 'label' });
  const select = label.parentElement!.querySelector('select');
  if (!select) throw new Error(`No select found for label "${labelText}"`);
  return select as HTMLSelectElement;
}

describe('ReconciliationScreen', () => {
  it('default view shows "Open exceptions" filtered to 2 unassigned records', () => {
    renderScreen();
    // Default filter is "Open exceptions" → owner is null → REC-002, REC-005
    expect(screen.getByText(/2\s+exception/)).toBeInTheDocument();
  });

  it('changing exception type to "All records" shows all 5 records', () => {
    renderScreen();
    const exceptionSelect = getSelectByLabel('Exception type');
    fireEvent.change(exceptionSelect, { target: { value: 'All records' } });
    expect(screen.getByText(/5\s+exception/)).toBeInTheDocument();
  });

  it('severity filter "High" shows only high severity records matching current exception type', () => {
    renderScreen();
    const exceptionSelect = getSelectByLabel('Exception type');
    const severitySelect = getSelectByLabel('Severity');
    // Switch to all records first, then filter by High
    fireEvent.change(exceptionSelect, { target: { value: 'All records' } });
    fireEvent.change(severitySelect, { target: { value: 'High' } });
    // 3 high severity records total: REC-001, REC-002, REC-005
    expect(screen.getByText(/3\s+exception/)).toBeInTheDocument();
  });

  it('severity filter "Medium" shows only medium severity records', () => {
    renderScreen();
    const exceptionSelect = getSelectByLabel('Exception type');
    const severitySelect = getSelectByLabel('Severity');
    // Switch to all records first, then filter by Medium
    fireEvent.change(exceptionSelect, { target: { value: 'All records' } });
    fireEvent.change(severitySelect, { target: { value: 'Medium' } });
    // 2 medium severity records: REC-003, REC-004
    expect(screen.getByText(/2\s+exception/)).toBeInTheDocument();
  });

  it('negative difference values have danger color class', () => {
    renderScreen();
    // Default shows open exceptions with negative differences (REC-002: -150M, REC-005: -180M)
    const dangerElements = document.querySelectorAll('.text-danger');
    expect(dangerElements.length).toBeGreaterThan(0);
    // Verify the text contains the negative sign format
    const hasNegativeValue = Array.from(dangerElements).some((el) =>
      el.textContent?.startsWith('- IDR'),
    );
    expect(hasNegativeValue).toBe(true);
  });

  it('positive difference values have success color class', () => {
    renderScreen();
    const exceptionSelect = getSelectByLabel('Exception type');
    // Switch to all records to include REC-003 which has positive difference (+50M)
    fireEvent.change(exceptionSelect, { target: { value: 'All records' } });
    const successElements = document.querySelectorAll('.text-success');
    expect(successElements.length).toBeGreaterThan(0);
    // Verify the text contains the positive sign format
    const hasPositiveValue = Array.from(successElements).some((el) =>
      el.textContent?.startsWith('+ IDR'),
    );
    expect(hasPositiveValue).toBe(true);
  });

  it('empty state renders message when filters produce zero results', () => {
    renderScreen();
    const severitySelect = getSelectByLabel('Severity');
    // Default is "Open exceptions" (owner null) → REC-002 (high), REC-005 (high)
    // Filter by Medium severity on open exceptions → 0 results (no medium with owner null)
    fireEvent.change(severitySelect, { target: { value: 'Medium' } });
    expect(screen.getByText('No exceptions match the current filters.')).toBeInTheDocument();
  });

  it('PageHeader shows title "Reconciliation"', () => {
    renderScreen();
    expect(screen.getByText('Reconciliation')).toBeInTheDocument();
  });

  it('NoticeBanner shows "Cutoff at 14:00 WIB"', () => {
    renderScreen();
    expect(screen.getByText('Cutoff at 14:00 WIB')).toBeInTheDocument();
  });
});
