import { describe, expect, it } from "vitest";

import type { AppointmentBody, RowBody } from "../../api/client";
import {
  countByStatus,
  effectiveState,
  filterRows,
  rowDate,
  rowTitle,
  sortRows,
  type BookingContext,
  type Criteria,
} from "./filter";

const booked: RowBody = {
  nre: "0900A0000000001",
  fse: {
    id: "f1",
    nre: "0900A0000000001",
    description: "Visita esempio",
    kind: "visit",
    status: 30,
    date: "2026-11-04T09:00:00Z",
    documentRef: "ref-1",
  },
};

const closedDrug: RowBody = {
  nre: "0900A0000000002",
  fse: {
    id: "f2",
    nre: "0900A0000000002",
    description: "Farmaco esempio",
    kind: "drug",
    status: 8,
    date: "2026-10-01T09:00:00Z",
    documentRef: "ref-2",
  },
};

const cupOnly: RowBody = {
  nre: "0900A0000000003",
  cup: {
    nre: "0900A0000000003",
    kind: "visit",
    status: 3,
    items: ["Ecografia esempio", "Esame sangue"],
    prescribedOn: "2026-11-05T09:00:00Z",
  },
};

const nullItems: RowBody = {
  nre: "0900A0000000004",
  cup: { nre: "0900A0000000004", kind: "visit", status: 3, items: null },
};

const rows = [booked, closedDrug, cupOnly, nullItems];

const base: Criteria = { status: "all", kind: "all", search: "" };

const now = new Date("2026-11-10T09:00:00Z");

const noBookings: BookingContext = {
  bookingByNre: new Map(),
  notInCup: new Set(),
  now,
};

function booking(
  nre: string,
  status: string,
  at: string | undefined,
): AppointmentBody {
  return {
    id: `a-${nre}`,
    nre,
    status,
    ...(at === undefined ? {} : { at }),
    unit: {
      name: "Unita esempio",
      description: "",
      address: "",
      city: "",
      province: "",
    },
  };
}

describe("filterRows", () => {
  it("bookable keeps rows with cup", () => {
    const got = filterRows(rows, { ...base, status: "bookable" }, noBookings);
    expect(got).toEqual([cupOnly, nullItems]);
  });

  it("open keeps non closed fse and cup only rows", () => {
    const got = filterRows(rows, { ...base, status: "open" }, noBookings);
    expect(got).toEqual([booked, cupOnly, nullItems]);
  });

  it("all keeps everything", () => {
    expect(filterRows(rows, base, noBookings)).toEqual(rows);
  });

  it("searches the description case-insensitively", () => {
    const got = filterRows(rows, { ...base, search: "FARMACO" }, noBookings);
    expect(got).toEqual([closedDrug]);
  });

  it("searches the cup items", () => {
    const got = filterRows(rows, { ...base, search: "ecografia" }, noBookings);
    expect(got).toEqual([cupOnly]);
  });

  it("searches the NRE", () => {
    const got = filterRows(
      rows,
      { ...base, search: "0900a0000000001" },
      noBookings,
    );
    expect(got).toEqual([booked]);
  });

  it("ignores a blank search", () => {
    expect(filterRows(rows, { ...base, search: "   " }, noBookings)).toEqual(
      rows,
    );
  });

  it("kind visit keeps visits and cup rows", () => {
    const got = filterRows(rows, { ...base, kind: "visit" }, noBookings);
    expect(got).toEqual([booked, cupOnly, nullItems]);
  });

  it("kind drug keeps drugs only", () => {
    const got = filterRows(rows, { ...base, kind: "drug" }, noBookings);
    expect(got).toEqual([closedDrug]);
  });

  it("handles null cup items", () => {
    const got = filterRows([nullItems], { ...base, search: "x" }, noBookings);
    expect(got).toEqual([]);
  });
});

describe("countByStatus", () => {
  it("counts each status filter", () => {
    expect(countByStatus(rows, "all", "", noBookings)).toEqual({
      bookable: 2,
      open: 3,
      all: 4,
    });
  });

  it("applies kind and search before counting", () => {
    expect(countByStatus(rows, "visit", "esempio", noBookings)).toEqual({
      bookable: 1,
      open: 2,
      all: 2,
    });
  });
});

describe("sortRows", () => {
  it("sorts by date descending using the cup date as fallback", () => {
    expect(sortRows([closedDrug, booked, cupOnly])).toEqual([
      cupOnly,
      booked,
      closedDrug,
    ]);
  });

  it("puts undated rows last", () => {
    expect(sortRows([nullItems, closedDrug, cupOnly])).toEqual([
      cupOnly,
      closedDrug,
      nullItems,
    ]);
  });

  it("does not mutate the input", () => {
    const input = [closedDrug, booked];
    sortRows(input);
    expect(input).toEqual([closedDrug, booked]);
  });
});

describe("rowDate", () => {
  it("prefers the fse date over the cup date", () => {
    const row: RowBody = {
      ...booked,
      cup: {
        nre: booked.nre,
        kind: "visit",
        status: 3,
        items: null,
        prescribedOn: "2026-12-01T09:00:00Z",
      },
    };
    expect(rowDate(row)).toBe("2026-11-04T09:00:00Z");
  });

  it("falls back to the cup date", () => {
    expect(rowDate(cupOnly)).toBe("2026-11-05T09:00:00Z");
  });
});

describe("rowTitle", () => {
  it("uses the fse description", () => {
    expect(rowTitle(booked)).toBe("Visita esempio");
  });

  it("joins the cup items", () => {
    expect(rowTitle(cupOnly)).toBe("Ecografia esempio, Esame sangue");
  });

  it("falls back to a generic title for null items", () => {
    expect(rowTitle(nullItems)).toBe("Prescrizione");
  });
});

describe("effectiveState", () => {
  it("is booked when a live booking exists, beating the fse status", () => {
    const bookings = new Map([
      [
        "0900A0000000001",
        booking("0900A0000000001", "PRENOTATO", "2026-12-01T09:00:00Z"),
      ],
    ]);
    expect(effectiveState(booked, bookings, new Set())).toEqual({
      kind: "booked",
      at: "2026-12-01T09:00:00Z",
      status: "PRENOTATO",
    });
  });

  it("upper-cases the row NRE for the lookup", () => {
    const row: RowBody = { ...cupOnly, nre: "0900a0000000003" };
    const bookings = new Map([
      ["0900A0000000003", booking("0900A0000000003", "PRENOTATO", undefined)],
    ]);
    expect(effectiveState(row, bookings, new Set())).toEqual({
      kind: "booked",
      at: undefined,
      status: "PRENOTATO",
    });
  });

  it("falls back to official when the booking is DISDETTO", () => {
    const bookings = new Map([
      [
        "0900A0000000001",
        booking("0900A0000000001", "DISDETTO", "2026-12-01T09:00:00Z"),
      ],
    ]);
    expect(effectiveState(booked, bookings, new Set())).toEqual({
      kind: "official",
      status: 30,
    });
  });

  it("is booked elsewhere when the NRE is not in the CUP", () => {
    expect(
      effectiveState(cupOnly, new Map(), new Set(["0900A0000000003"])),
    ).toEqual({ kind: "booked_elsewhere" });
  });

  it("is official with the fse status", () => {
    expect(effectiveState(closedDrug, new Map(), new Set())).toEqual({
      kind: "official",
      status: 8,
    });
  });

  it("is official with the to-book status for a cup only row", () => {
    expect(effectiveState(cupOnly, new Map(), new Set())).toEqual({
      kind: "official",
      status: 3,
    });
  });
});

describe("filterRows with bookings", () => {
  const context: BookingContext = {
    bookingByNre: new Map([
      [
        "0900A0000000003",
        booking("0900A0000000003", "PRENOTATO", "2026-12-01T09:00:00Z"),
      ],
      [
        "0900A0000000001",
        booking("0900A0000000001", "PRENOTATO", "2026-11-01T09:00:00Z"),
      ],
    ]),
    notInCup: new Set(["0900A0000000004"]),
    now,
  };

  it("bookable excludes booked and booked elsewhere rows", () => {
    const got = filterRows(rows, { ...base, status: "bookable" }, context);
    expect(got).toEqual([]);
  });

  it("open counts a future booked row even when its fse status is closed", () => {
    const closedBooked: RowBody = { ...closedDrug, nre: "0900A0000000003" };
    const got = filterRows(
      [closedBooked],
      { ...base, status: "open" },
      context,
    );
    expect(got).toEqual([closedBooked]);
  });

  it("open falls back to the fse status for a past booked row", () => {
    const pastClosed: RowBody = { ...closedDrug, nre: "0900A0000000001" };
    const got = filterRows([pastClosed], { ...base, status: "open" }, context);
    expect(got).toEqual([]);
  });

  it("countByStatus applies the booking context", () => {
    expect(countByStatus(rows, "all", "", context)).toEqual({
      bookable: 0,
      open: 3,
      all: 4,
    });
  });
});
