import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { ImportChange } from "../api";
import { ImportChangeTable } from "./ImportChangeTable";

function makeChanges(): ImportChange[] {
  const updates: ImportChange[] = Array.from({ length: 60 }, (_, i) => ({
    row: i + 2,
    op: "update",
    key: `K${i}`,
    fields: [{ field: "name", before: "lama", after: "baru" }],
  }));
  return [
    ...updates,
    {
      row: 70,
      op: "disable",
      key: "KD",
      fields: [{ field: "Status", before: "Aktif", after: "Nonaktif" }],
    },
  ];
}

describe("ImportChangeTable", () => {
  it("pages 50 rows at a time", async () => {
    render(<ImportChangeTable changes={makeChanges()} />);
    expect(screen.getByText("Menampilkan 1-50 dari 61 perubahan")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Berikutnya" }));
    expect(screen.getByText("Menampilkan 51-61 dari 61 perubahan")).toBeInTheDocument();
  });

  it("filter returns to page 1 and narrows rows", async () => {
    render(<ImportChangeTable changes={makeChanges()} />);
    await userEvent.click(screen.getByRole("button", { name: "Berikutnya" }));
    await userEvent.selectOptions(screen.getByLabelText("Operasi"), "disable");
    expect(screen.getByText("Menampilkan 1-1 dari 1 perubahan")).toBeInTheDocument();
    expect(screen.getByText("KD")).toBeInTheDocument();
  });

  it("renders values as text, not HTML", () => {
    render(
      <ImportChangeTable
        changes={[
          {
            row: 2,
            op: "create",
            key: "X",
            fields: [{ field: "name", before: "", after: "<b>bold</b>" }],
          },
        ]}
      />,
    );
    expect(screen.getByText("<b>bold</b>")).toBeInTheDocument();
  });
});
