import { describe, it, expect, vi, beforeAll } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Sidebar } from '../Sidebar';

// Mock window.matchMedia
beforeAll(() => {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  });
});

const defaultProps = {
  collapsed: false,
  onToggle: vi.fn(),
  mobileOpen: false,
  onMobileClose: vi.fn(),
};

function renderSidebar(props = {}) {
  return render(
    <MemoryRouter initialEntries={['/dashboard']}>
      <Sidebar {...defaultProps} {...props} />
    </MemoryRouter>
  );
}

describe('Sidebar', () => {
  describe('NavGroup labels', () => {
    it('renders "Operations" and "Control" group labels when expanded', () => {
      renderSidebar();

      expect(screen.getByText('Operations')).toBeInTheDocument();
      expect(screen.getByText('Control')).toBeInTheDocument();
    });

    it('applies uppercase styling to group labels', () => {
      renderSidebar();

      const operations = screen.getByText('Operations');
      const control = screen.getByText('Control');

      expect(operations).toHaveClass('uppercase');
      expect(control).toHaveClass('uppercase');
    });

    it('does not render group labels when collapsed', () => {
      renderSidebar({ collapsed: true });

      expect(screen.queryByText('Operations')).not.toBeInTheDocument();
      expect(screen.queryByText('Control')).not.toBeInTheDocument();
    });
  });

  describe('Brand Mark', () => {
    it('renders "CN" logo mark', () => {
      renderSidebar();

      expect(screen.getByText('CN')).toBeInTheDocument();
    });

    it('renders "CodexCash" brand text when expanded', () => {
      renderSidebar();

      expect(screen.getByText('CodexCash')).toBeInTheDocument();
    });

    it('hides "CodexCash" brand text when collapsed (opacity 0)', () => {
      renderSidebar({ collapsed: true });

      const brandText = screen.getByText('CodexCash');
      expect(brandText.parentElement).toHaveStyle({ opacity: '0', width: '0' });
    });
  });

  describe('Navigation items', () => {
    it('renders all expected nav items', () => {
      renderSidebar();

      const expectedItems = [
        'Dashboard',
        'Replenishment',
        'Cash Count',
        'Reconciliation',
        'Vendor Invoices',
        'DSR Reports',
        'Forecasting',
        'Settings',
      ];

      for (const label of expectedItems) {
        expect(screen.getByText(label)).toBeInTheDocument();
      }
    });
  });

  describe('Active state', () => {
    it('applies active styling to the nav item matching current route', () => {
      renderSidebar();

      const dashboardLink = screen.getByRole('link', { name: /Dashboard/i });
      expect(dashboardLink).toHaveClass('bg-[var(--red-50)]');
      expect(dashboardLink).toHaveClass('text-[var(--red-600)]');
    });

    it('does not apply active styling to non-matching nav items', () => {
      renderSidebar();

      const settingsLink = screen.getByRole('link', { name: /Settings/i });
      expect(settingsLink).not.toHaveClass('bg-[var(--red-50)]');
      expect(settingsLink).toHaveClass('text-[var(--n-600)]');
    });
  });

  describe('Collapsed behavior', () => {
    it('hides nav item labels when collapsed (opacity 0, width 0)', () => {
      renderSidebar({ collapsed: true });

      const dashboard = screen.getByText('Dashboard');
      expect(dashboard).toHaveStyle({ opacity: '0', width: '0' });
    });

    it('sets sidebar width to 64px when collapsed', () => {
      const { container } = renderSidebar({ collapsed: true });

      const aside = container.querySelector('aside');
      expect(aside).toHaveStyle({ width: '64px' });
    });

    it('sets sidebar width to 256px when expanded', () => {
      const { container } = renderSidebar();

      const aside = container.querySelector('aside');
      expect(aside).toHaveStyle({ width: '256px' });
    });
  });

  describe('Badge display', () => {
    it('does not render badge when badge count is 0 or undefined', () => {
      renderSidebar();

      // With default constants, no badges are set (all undefined -> treated as 0)
      const badgePills = screen.queryAllByText(/^\d+$|^99\+$/);
      expect(badgePills).toHaveLength(0);
    });
  });

  describe('System status note', () => {
    it('renders system status text when expanded', () => {
      renderSidebar();

      expect(screen.getByText('All systems operational')).toBeInTheDocument();
    });

    it('does not render system status text when collapsed', () => {
      renderSidebar({ collapsed: true });

      expect(screen.queryByText('All systems operational')).not.toBeInTheDocument();
    });
  });

  describe('Mobile overlay', () => {
    it('renders mobile overlay when mobileOpen is true', () => {
      const { container } = renderSidebar({ mobileOpen: true });

      const overlay = container.querySelector('.fixed.inset-0');
      expect(overlay).toBeInTheDocument();
    });

    it('does not render mobile overlay when mobileOpen is false', () => {
      const { container } = renderSidebar({ mobileOpen: false });

      const overlay = container.querySelector('.fixed.inset-0');
      expect(overlay).not.toBeInTheDocument();
    });
  });
});
