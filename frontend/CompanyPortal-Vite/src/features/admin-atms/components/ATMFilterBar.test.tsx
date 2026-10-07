import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { AdminATMsUrlParams } from "../useAdminATMsUrlState";
import { ATMFilterBar } from "./ATMFilterBar";

const BASE_PARAMS: AdminATMsUrlParams = {
  page: 1,
  page_size: 25,
  status: "active",
  q: "",
  brand: "",
  machine_type: "",
  deployment_type: "",
  priority_class: "",
};

describe("ATMFilterBar", () => {
  it("typing in the search box calls onSearchInputChange", async () => {
    const user = userEvent.setup();
    const onSearchInputChange = vi.fn();
    render(
      <ATMFilterBar
        params={BASE_PARAMS}
        searchInput=""
        onSearchInputChange={onSearchInputChange}
        onParamsChange={vi.fn()}
      />,
    );

    await user.type(screen.getByLabelText("Cari"), "T");
    expect(onSearchInputChange).toHaveBeenCalledWith("T");
  });

  it("changing Brand resets page to 1", async () => {
    const user = userEvent.setup();
    const onParamsChange = vi.fn();
    render(
      <ATMFilterBar
        params={BASE_PARAMS}
        searchInput=""
        onSearchInputChange={vi.fn()}
        onParamsChange={onParamsChange}
      />,
    );

    await user.selectOptions(screen.getByLabelText("Brand"), "NCR");
    expect(onParamsChange).toHaveBeenCalledWith({ brand: "NCR", page: 1 });
  });

  it("changing Tipe Mesin resets page to 1", async () => {
    const user = userEvent.setup();
    const onParamsChange = vi.fn();
    render(
      <ATMFilterBar
        params={BASE_PARAMS}
        searchInput=""
        onSearchInputChange={vi.fn()}
        onParamsChange={onParamsChange}
      />,
    );

    await user.selectOptions(screen.getByLabelText("Tipe Mesin"), "CDM");
    expect(onParamsChange).toHaveBeenCalledWith({ machine_type: "CDM", page: 1 });
  });

  it("changing Deployment resets page to 1", async () => {
    const user = userEvent.setup();
    const onParamsChange = vi.fn();
    render(
      <ATMFilterBar
        params={BASE_PARAMS}
        searchInput=""
        onSearchInputChange={vi.fn()}
        onParamsChange={onParamsChange}
      />,
    );

    await user.selectOptions(screen.getByLabelText("Deployment"), "OFFSITE");
    expect(onParamsChange).toHaveBeenCalledWith({ deployment_type: "OFFSITE", page: 1 });
  });

  it("changing Prioritas resets page to 1", async () => {
    const user = userEvent.setup();
    const onParamsChange = vi.fn();
    render(
      <ATMFilterBar
        params={BASE_PARAMS}
        searchInput=""
        onSearchInputChange={vi.fn()}
        onParamsChange={onParamsChange}
      />,
    );

    await user.selectOptions(screen.getByLabelText("Prioritas"), "VIP");
    expect(onParamsChange).toHaveBeenCalledWith({ priority_class: "VIP", page: 1 });
  });

  it("clearing a filter select back to 'Semua' sends an empty string", async () => {
    const user = userEvent.setup();
    const onParamsChange = vi.fn();
    render(
      <ATMFilterBar
        params={{ ...BASE_PARAMS, brand: "NCR" }}
        searchInput=""
        onSearchInputChange={vi.fn()}
        onParamsChange={onParamsChange}
      />,
    );

    await user.selectOptions(screen.getByLabelText("Brand"), "Semua");
    expect(onParamsChange).toHaveBeenCalledWith({ brand: "", page: 1 });
  });
});
