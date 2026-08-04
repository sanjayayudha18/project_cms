import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ToastProvider } from '@/context/ToastContext';
import { DashboardScreen } from '../DashboardScreen';

function renderDashboard() {
  return render(
    <MemoryRouter>
      <ToastProvider>
        <DashboardScreen />
      </ToastProvider>
    </MemoryRouter>,
  );
}

describe('DashboardScreen', () => {
  describe('MetricStrip', () => {
    it('renders all 4 metric labels', () => {
      renderDashboard();

      expect(screen.getByText('Managed Cash')).toBeInTheDocument();
      expect(screen.getByText('ATM Availability')).toBeInTheDocument();
      expect(screen.getByText("Today's Routes")).toBeInTheDocument();
      expect(screen.getByText('Exceptions')).toBeInTheDocument();
    });
  });

  describe('AttentionPanel', () => {
    it('renders with "Needs attention" heading', () => {
      renderDashboard();

      expect(
        screen.getByRole('heading', { name: 'Needs attention' }),
      ).toBeInTheDocument();
    });

    it('shows attention items', () => {
      renderDashboard();

      expect(
        screen.getByText('Escrow mismatch on ATM-JKT-008'),
      ).toBeInTheDocument();
    });
  });

  describe('ReplenishmentSummary', () => {
    it('renders "Today\'s replenishment" heading', () => {
      renderDashboard();

      expect(
        screen.getByRole('heading', { name: "Today's replenishment" }),
      ).toBeInTheDocument();
    });

    it('has "View all" link pointing to /replenishment', () => {
      renderDashboard();

      const link = screen.getByRole('link', { name: /view all/i });
      expect(link).toBeInTheDocument();
      expect(link).toHaveAttribute('href', '/replenishment');
    });
  });

  describe('Action buttons', () => {
    it('renders "New schedule" button', () => {
      renderDashboard();

      expect(
        screen.getByRole('button', { name: 'New schedule' }),
      ).toBeInTheDocument();
    });

    it('renders "Export DSR" button', () => {
      renderDashboard();

      expect(
        screen.getByRole('button', { name: 'Export DSR' }),
      ).toBeInTheDocument();
    });
  });
});
