import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { StatusOptionBody } from "../api/client";
import { render } from "../test/render";
import { StatusKeyFilter } from "./StatusKeyFilter";

const options: StatusOptionBody[] = [
  {
    key: "booked",
    label: "Prenotata",
    tone: "booked",
    count: 2,
    excluded: false,
  },
  { key: "done", label: "Erogata", tone: "done", count: 1, excluded: true },
  {
    key: "expired",
    label: "Scaduta",
    tone: "closed",
    count: 3,
    excluded: false,
  },
];

describe("StatusKeyFilter", () => {
  it("checks the options that are not excluded", () => {
    render(<StatusKeyFilter options={options} onChange={vi.fn()} />);

    expect(
      screen.getByRole("checkbox", { name: "Prenotata (2)" }),
    ).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: "Erogata (1)" }),
    ).not.toBeChecked();
  });

  it("reports the excluded keys when unticking a box", async () => {
    const onChange = vi.fn();
    render(<StatusKeyFilter options={options} onChange={onChange} />);

    await userEvent.click(
      screen.getByRole("checkbox", { name: "Prenotata (2)" }),
    );

    expect(onChange).toHaveBeenCalledWith(["booked", "done"]);
  });

  it("reports the excluded keys when ticking a box", async () => {
    const onChange = vi.fn();
    render(<StatusKeyFilter options={options} onChange={onChange} />);

    await userEvent.click(
      screen.getByRole("checkbox", { name: "Erogata (1)" }),
    );

    expect(onChange).toHaveBeenCalledWith([]);
  });

  it("shows everything with Tutti", async () => {
    const onChange = vi.fn();
    render(<StatusKeyFilter options={options} onChange={onChange} />);

    await userEvent.click(screen.getByRole("button", { name: "Tutti" }));

    expect(onChange).toHaveBeenCalledWith([]);
  });

  it("excludes every key with Nessuno", async () => {
    const onChange = vi.fn();
    render(<StatusKeyFilter options={options} onChange={onChange} />);

    await userEvent.click(screen.getByRole("button", { name: "Nessuno" }));

    expect(onChange).toHaveBeenCalledWith(["booked", "done", "expired"]);
  });
});
