import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect } from 'vitest';

import { ToastProvider } from '@/context/ToastContext';
import { ReplenishmentScreen } from '../ReplenishmentScreen';

function renderScreen() {
  return render(
    <MemoryRouter>
      <ToastProvider>
        <ReplenishmentScreen />
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

describe('ReplenishmentScreen', () => {
  it('renders all 8 schedules initially', () => {
    renderScreen();
    expect(screen.getByText(/8\s+schedules/)).toBeInTheDocument();
  });

  it('selecting a region filter updates the count', () => {
    renderScreen();
    const regionSelect = getSelectByLabel('Region');
    fireEvent.change(regionSelect, { target: { value: 'South Jakarta' } });
    expect(screen.getByText(/2\s+schedules/)).toBeInTheDocument();
  });

  it('selecting a vendor filter updates the count', () => {
    renderScreen();
    const vendorSelect = getSelectByLabel('Vendor');
    fireEvent.change(vendorSelect, { target: { value: 'PT Gardanet' } });
    expect(screen.getByText(/3\s+schedules/)).toBeInTheDocument();
  });

  it('combined region + vendor filters narrow results', () => {
    renderScreen();
    const regionSelect = getSelectByLabel('Region');
    const vendorSelect = getSelectByLabel('Vendor');
    fireEvent.change(regionSelect, { target: { value: 'South Jakarta' } });
    fireEvent.change(vendorSelect, { target: { value: 'PT Gardanet' } });
    expect(screen.getByText(/1\s+schedules/)).toBeInTheDocument();
  });

  it('shows empty state when filters produce zero results', () => {
    renderScreen();
    const regionSelect = getSelectByLabel('Region');
    const vendorSelect = getSelectByLabel('Vendor');
    // North Jakarta has PT SSI and PT Gardanet, no PT G4S
    fireEvent.change(regionSelect, { target: { value: 'North Jakarta' } });
    fireEvent.change(vendorSelect, { target: { value: 'PT G4S' } });
    expect(screen.getByText('No schedules found')).toBeInTheDocument();
  });

  it('renders "New schedule" button', () => {
    renderScreen();
    expect(screen.getByRole('button', { name: /new schedule/i })).toBeInTheDocument();
  });

  it('PageHeader shows title "Replenishment schedules"', () => {
    renderScreen();
    expect(screen.getByText('Replenishment schedules')).toBeInTheDocument();
  });
});
