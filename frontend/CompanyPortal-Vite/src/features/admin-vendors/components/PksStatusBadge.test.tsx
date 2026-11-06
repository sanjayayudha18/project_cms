import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { PksStatus } from "../types";
import { PksStatusBadge } from "./PksStatusBadge";

const LABELS: Array<[PksStatus, string]> = [
  ["aktif", "Aktif"],
  ["need_renewal", "Need Renewal"],
  ["expired", "Expired"],
  ["terminated", "Terminated"],
  ["belum_diisi", "Belum diisi"],
];

describe("PksStatusBadge", () => {
  it.each(LABELS)("%s → label %s dengan ikon", (status, label) => {
    const { container } = render(<PksStatusBadge status={status} />);
    expect(screen.getByText(label)).toBeInTheDocument();
    expect(container.querySelector("svg")).not.toBeNull();
  });

  it("lima label berbeda", () => {
    expect(new Set(LABELS.map(([, l]) => l)).size).toBe(5);
  });

  it.each([null, undefined])("vendor internal (%s) → '-' tanpa ikon", (status) => {
    const { container } = render(<PksStatusBadge status={status} />);
    expect(screen.getByText("-")).toBeInTheDocument();
    expect(container.querySelector("svg")).toBeNull();
  });
});
