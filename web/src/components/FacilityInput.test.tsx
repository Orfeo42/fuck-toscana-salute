import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import type { JSX } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { render } from "../test/render";
import { FacilityInput } from "./FacilityInput";

const mocks = vi.hoisted(() => ({
  post: vi.fn(),
}));

vi.mock("../api/client", () => ({
  api: { POST: mocks.post },
}));

const facilities = [
  {
    name: "Ospedale Esempio",
    comune: "Comune Finto",
    provincia: "XX",
    address: "Via Prova 1",
    source: "ministero",
  },
  {
    name: "Ospedale Esempio",
    comune: "Altro Comune",
    provincia: "YY",
    address: "",
    source: "regione",
  },
];

function Harness({
  onChange,
}: {
  onChange: (value: string) => void;
}): JSX.Element {
  const [value, setValue] = useState("");
  return (
    <FacilityInput
      label="Struttura"
      value={value}
      onChange={(next) => {
        setValue(next);
        onChange(next);
      }}
    />
  );
}

describe("FacilityInput", () => {
  beforeEach(() => {
    mocks.post.mockReset();
    mocks.post.mockResolvedValue({
      data: { facilities },
      response: { status: 200 },
    });
  });

  it("does not search with a single character", async () => {
    render(<Harness onChange={vi.fn()} />);

    await userEvent.type(
      screen.getByRole("combobox", { name: "Struttura" }),
      "O",
    );
    await new Promise((resolve) => setTimeout(resolve, 400));

    expect(mocks.post).not.toHaveBeenCalled();
  });

  it("posts the typed text once after the debounce and renders the options", async () => {
    render(<Harness onChange={vi.fn()} />);

    await userEvent.type(
      screen.getByRole("combobox", { name: "Struttura" }),
      "Osp",
    );

    expect(
      await screen.findByText("Comune Finto (XX) · Via Prova 1"),
    ).toBeVisible();
    expect(screen.getByText("Altro Comune (YY)")).toBeVisible();
    expect(mocks.post).toHaveBeenCalledTimes(1);
    expect(mocks.post.mock.calls[0]?.[0]).toBe("/api/facilities/search");
    expect(mocks.post.mock.calls[0]?.[1]).toMatchObject({
      body: { query: "Osp" },
    });
  });

  it("calls onChange with the facility name when an option is picked", async () => {
    const onChange = vi.fn();
    render(<Harness onChange={onChange} />);

    await userEvent.type(
      screen.getByRole("combobox", { name: "Struttura" }),
      "Osp",
    );
    await userEvent.click(await screen.findByText("Altro Comune (YY)"));

    expect(onChange).toHaveBeenLastCalledWith("Ospedale Esempio");
    expect(screen.getByRole("combobox", { name: "Struttura" })).toHaveValue(
      "Ospedale Esempio",
    );
  });

  it("keeps free text working", async () => {
    const onChange = vi.fn();
    render(<Harness onChange={onChange} />);

    await userEvent.type(
      screen.getByRole("combobox", { name: "Struttura" }),
      "Studio Mio",
    );

    expect(onChange).toHaveBeenLastCalledWith("Studio Mio");
    expect(screen.getByRole("combobox", { name: "Struttura" })).toHaveValue(
      "Studio Mio",
    );
  });

  it("keeps working when the request fails", async () => {
    mocks.post.mockRejectedValue(new Error("down"));
    const onChange = vi.fn();
    render(<Harness onChange={onChange} />);

    await userEvent.type(
      screen.getByRole("combobox", { name: "Struttura" }),
      "Osp",
    );
    await waitFor(() => {
      expect(mocks.post).toHaveBeenCalled();
    });

    expect(screen.getByRole("combobox", { name: "Struttura" })).toHaveValue(
      "Osp",
    );
    expect(screen.queryByText("Comune Finto (XX) · Via Prova 1")).toBeNull();
  });

  it("shows the data attribution", () => {
    render(<Harness onChange={vi.fn()} />);

    expect(
      screen.getByText(/Ministero della Salute \(IODL 2\.0\)/),
    ).toBeVisible();
  });
});
