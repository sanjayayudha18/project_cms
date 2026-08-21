import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { AtmLevelRow, AtmLevelTable, getCashLevelTier } from "../AtmLevelTable";
import type { AtmLevel } from "../types";

const mockLevels: AtmLevel[] = [
  { id: "ATM-00417", label: "ATM-00417", percentage: 82 },
  { id: "ATM-00189", label: "ATM-00189", percentage: 43 },
  { id: "ATM-00302", label: "ATM-00302", percentage: 15 },
];
const [atmHigh, , atmLow] = mockLevels;
if (!atmHigh || !atmLow) throw new Error("Expected mockLevels to have fixed fixture entries");

describe("getCashLevelTier", () => {
  it('returns "success" for percentage >= 50 (e.g. 82)', () => {
    expect(getCashLevelTier(82)).toBe("success");
  });

  it('returns "warning" for percentage 20–49 (e.g. 43)', () => {
    expect(getCashLevelTier(43)).toBe("warning");
  });

  it('returns "danger" for percentage < 20 (e.g. 15)', () => {
    expect(getCashLevelTier(15)).toBe("danger");
  });

  it('returns "success" at boundary 50', () => {
    expect(getCashLevelTier(50)).toBe("success");
  });

  it('returns "warning" at boundary 20', () => {
    expect(getCashLevelTier(20)).toBe("warning");
  });

  it('returns "danger" at boundary 19', () => {
    expect(getCashLevelTier(19)).toBe("danger");
  });
});

describe("AtmLevelTable", () => {
  it("renders all ATM rows", () => {
    render(<AtmLevelTable levels={mockLevels} />);

    expect(screen.getByText("ATM-00417")).toBeInTheDocument();
    expect(screen.getByText("ATM-00189")).toBeInTheDocument();
    expect(screen.getByText("ATM-00302")).toBeInTheDocument();
  });
});

describe("AtmLevelRow", () => {
  function renderRow(atm: AtmLevel) {
    return render(
      <table>
        <tbody>
          <AtmLevelRow atm={atm} />
        </tbody>
      </table>,
    );
  }

  it("renders ATM identifier with monospace font (font-mono class)", () => {
    renderRow(atmHigh);

    const label = screen.getByText("ATM-00417");
    expect(label).toHaveClass("font-mono");
  });

  it("renders percentage value with tabular-nums class", () => {
    renderRow(atmHigh);

    const pctCell = screen.getByText("82%");
    expect(pctCell).toHaveClass("tabular-nums");
  });

  it('renders progress bar with role="progressbar"', () => {
    renderRow(atmHigh);

    expect(screen.getByRole("progressbar")).toBeInTheDocument();
  });

  it("sets aria-valuenow matching the percentage", () => {
    renderRow(atmHigh);

    const progressbar = screen.getByRole("progressbar");
    expect(progressbar).toHaveAttribute("aria-valuenow", "82");
  });

  it("sets aria-valuemin=0 and aria-valuemax=100", () => {
    renderRow(atmHigh);

    const progressbar = screen.getByRole("progressbar");
    expect(progressbar).toHaveAttribute("aria-valuemin", "0");
    expect(progressbar).toHaveAttribute("aria-valuemax", "100");
  });

  it("applies success tier bg class for high percentage", () => {
    renderRow(atmHigh); // 82% -> success

    const progressbar = screen.getByRole("progressbar");
    const fill = progressbar.firstElementChild as HTMLElement;
    expect(fill).toHaveClass("bg-[var(--success-solid)]");
  });

  it("applies danger tier bg class for low percentage", () => {
    renderRow(atmLow); // 15% -> danger

    const progressbar = screen.getByRole("progressbar");
    const fill = progressbar.firstElementChild as HTMLElement;
    expect(fill).toHaveClass("bg-[var(--danger-500)]");
  });
});
