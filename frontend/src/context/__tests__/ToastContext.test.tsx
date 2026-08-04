import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import { ToastProvider, useToast } from '../ToastContext';

function TestConsumer({ text, icon }: { text: string; icon?: 'success' | 'info' | 'warning' }) {
  const { showToast } = useToast();
  return <button onClick={() => showToast(text, icon)}>Trigger</button>;
}

describe('ToastContext', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('showToast causes the toast to appear with the given text', () => {
    render(
      <ToastProvider>
        <TestConsumer text="Item saved" />
      </ToastProvider>,
    );

    expect(screen.queryByText('Item saved')).not.toBeInTheDocument();

    act(() => {
      screen.getByRole('button', { name: 'Trigger' }).click();
    });

    expect(screen.getByText('Item saved')).toBeInTheDocument();
  });

  it('toast auto-dismisses after 4 seconds', () => {
    render(
      <ToastProvider>
        <TestConsumer text="Auto dismiss" />
      </ToastProvider>,
    );

    act(() => {
      screen.getByRole('button', { name: 'Trigger' }).click();
    });

    expect(screen.getByText('Auto dismiss')).toBeInTheDocument();

    // Advance just under 4 seconds — still visible
    act(() => {
      vi.advanceTimersByTime(3999);
    });
    expect(screen.getByText('Auto dismiss')).toBeInTheDocument();

    // Advance to 4 seconds — dismiss starts (animate-out)
    act(() => {
      vi.advanceTimersByTime(1);
    });

    // After animation-out completes (220ms), toast is removed from DOM
    act(() => {
      vi.advanceTimersByTime(220);
    });
    expect(screen.queryByText('Auto dismiss')).not.toBeInTheDocument();
  });

  it('replaces old toast and resets timer when a new toast is triggered', () => {
    function MultiConsumer() {
      const { showToast } = useToast();
      return (
        <>
          <button onClick={() => showToast('First toast')}>First</button>
          <button onClick={() => showToast('Second toast')}>Second</button>
        </>
      );
    }

    render(
      <ToastProvider>
        <MultiConsumer />
      </ToastProvider>,
    );

    // Show first toast
    act(() => {
      screen.getByRole('button', { name: 'First' }).click();
    });
    expect(screen.getByText('First toast')).toBeInTheDocument();

    // Advance 3 seconds, then trigger second toast
    act(() => {
      vi.advanceTimersByTime(3000);
    });

    act(() => {
      screen.getByRole('button', { name: 'Second' }).click();
    });

    // Old toast is replaced
    expect(screen.queryByText('First toast')).not.toBeInTheDocument();
    expect(screen.getByText('Second toast')).toBeInTheDocument();

    // Timer is reset — second toast should survive another 3.9 seconds
    act(() => {
      vi.advanceTimersByTime(3900);
    });
    expect(screen.getByText('Second toast')).toBeInTheDocument();

    // After full 4 seconds + animation, second toast is gone
    act(() => {
      vi.advanceTimersByTime(100);
    });
    act(() => {
      vi.advanceTimersByTime(220);
    });
    expect(screen.queryByText('Second toast')).not.toBeInTheDocument();
  });

  it('toast has role="status" and aria-live="polite"', () => {
    render(
      <ToastProvider>
        <TestConsumer text="Accessible toast" />
      </ToastProvider>,
    );

    act(() => {
      screen.getByRole('button', { name: 'Trigger' }).click();
    });

    const toast = screen.getByRole('status');
    expect(toast).toBeInTheDocument();
    expect(toast).toHaveAttribute('aria-live', 'polite');
  });

  it('renders icon when provided', () => {
    render(
      <ToastProvider>
        <TestConsumer text="Success action" icon="success" />
      </ToastProvider>,
    );

    act(() => {
      screen.getByRole('button', { name: 'Trigger' }).click();
    });

    // Icon should be present with aria-hidden (Lucide icons render as SVG)
    const toast = screen.getByRole('status');
    const icon = toast.querySelector('svg[aria-hidden="true"]');
    expect(icon).toBeInTheDocument();
  });
});
