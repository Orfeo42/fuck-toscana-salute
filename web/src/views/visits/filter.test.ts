import { describe, expect, it } from "vitest";

import type { AppointmentBody, RowBody } from "../../api/client";
import {
  buildRows,
  countByStatus,
  countExpired,
  effectiveState,
  filterRows,
  rowDate,
  rowTitle,
  sortRows,
  unitName,
  type BookingContext,
  type Criteria,
} from "./filter";

function fseRow(
  nre: string,
  kind: string,
  status: number,
  date: string,
  description = "Visita esempio",
): RowBody {
  return {
    nre,
    fse: {
      id: `f-${nre}`,
      nre,
      description,
      kind,
      status,
      date,
      documentRef: "",
    },
  };
}

const toBook = fseRow("0900A0000000001", "visit", 3, "2026-11-04T09:00:00Z");
const pending = fseRow("0900A0000000002", "visit", 5, "2026-11-01T09:00:00Z");
const done = fseRow("0900A0000000003", "visit", 8, "2026-10-01T09:00:00Z");
const closed = fseRow("0900A0000000004", "visit", 4, "2026-09-01T09:00:00Z");
const drug = fseRow("0900A0000000005", "drug", 5, "2026-11-02T09:00:00Z");
const cupOnly: RowBody = {
  nre: "0900A0000000006",
  cup: {
    nre: "0900A0000000006",
    kind: "visit",
    status: 3,
    items: ["Ecografia esempio", "Esame sangue"],
    prescribedOn: "2026-11-05T09:00:00Z",
  },
};
const nullItems: RowBody = {
  nre: "0900A0000000007",
  cup: { nre: "0900A0000000007", kind: "visit", status: 3, items: null },
};

const now = new Date("2026-11-10T09:00:00Z");

function booking(
  nre: string,
  status: string,
  at: string | undefined,
  city = "Firenze",
): AppointmentBody {
  return {
    id: `a-${nre}`,
    nre,
    status,
    ...(at === undefined ? {} : { at }),
    unit: {
      name: "Unita esempio",
      description: "",
      address: "Via Esempio 1",
      city,
      province: "FI",
    },
  };
}

function contextOf(
  bookings: AppointmentBody[],
  unbooked: string[] = [],
): BookingContext {
  return {
    bookingByNre: new Map(bookings.map((b) => [b.nre, b])),
    unbooked: new Set(unbooked),
    now,
  };
}

const empty = contextOf([]);
const base: Criteria = { status: "all", search: "", showExpired: false };

describe("effectiveState", () => {
  it("is booked with the booked tone and label, beating the fse status", () => {
    const bookings = new Map([
      [
        "0900A0000000002",
        booking("0900A0000000002", "PRENOTATO", "2026-12-01T09:00:00Z"),
      ],
    ]);
    expect(effectiveState(pending, bookings, new Set(), now)).toEqual({
      kind: "booked",
      at: "2026-12-01T09:00:00Z",
      status: "PRENOTATO",
      label: "Prenotata",
      tone: "booked",
    });
  });

  it("upper-cases the row NRE for the lookup", () => {
    const row: RowBody = { ...cupOnly, nre: "0900a0000000006" };
    const bookings = new Map([
      ["0900A0000000006", booking("0900A0000000006", "PRENOTATO", undefined)],
    ]);
    expect(effectiveState(row, bookings, new Set(), now).kind).toBe("booked");
  });

  it("keeps the official state when the only booking is DISDETTO", () => {
    const bookings = new Map([
      [
        "0900A0000000001",
        booking("0900A0000000001", "DISDETTO", "2026-12-01T09:00:00Z"),
      ],
    ]);
    expect(effectiveState(toBook, bookings, new Set(), now)).toEqual({
      kind: "official",
      status: 3,
      label: "Da prenotare",
      tone: "tobook",
      note: undefined,
      canBook: true,
    });
  });

  it("marks FSE status 30 as to verify when the CUP has no appointment", () => {
    const row = fseRow("0900A0000000010", "visit", 30, "2026-11-01T09:00:00Z");
    expect(
      effectiveState(row, new Map(), new Set(["0900A0000000010"]), now),
    ).toEqual({
      kind: "official",
      status: 30,
      label: "Da verificare",
      tone: "pending",
      note: "FSE la segna come prenotata, ma nel CUP non c'è un tuo appuntamento: può essere già gestita altrove (prenotazione o accettazione) o ancora prenotabile",
      canBook: true,
    });
  });

  it("keeps FSE status 30 as booked without a note when not unbooked", () => {
    const row = fseRow("0900A0000000010", "visit", 30, "2026-11-01T09:00:00Z");
    expect(
      effectiveState(row, new Map(), new Set(["0900A0000000099"]), now),
    ).toEqual({
      kind: "official",
      status: 30,
      label: "Prenotata",
      tone: "booked",
      note: undefined,
      canBook: false,
    });
  });

  it("maps status 3 to tobook, also for a cup only row", () => {
    expect(effectiveState(cupOnly, new Map(), new Set(), now)).toMatchObject({
      status: 3,
      tone: "tobook",
    });
  });

  it("maps status 5 to pending", () => {
    expect(effectiveState(pending, new Map(), new Set(), now)).toMatchObject({
      label: "Presa in carico",
      tone: "pending",
    });
  });

  it("maps status 30 to booked", () => {
    const row = fseRow("0900A0000000010", "visit", 30, "2026-11-01T09:00:00Z");
    expect(effectiveState(row, new Map(), new Set(), now)).toMatchObject({
      label: "Prenotata",
      tone: "booked",
    });
  });

  it("maps status 7 to pending", () => {
    const row = fseRow("0900A0000000011", "visit", 7, "2026-11-01T09:00:00Z");
    expect(effectiveState(row, new Map(), new Set(), now).tone).toBe("pending");
  });

  it("maps statuses 8 and 9 to done", () => {
    const after = fseRow("0900A0000000012", "visit", 9, "2026-11-01T09:00:00Z");
    expect(effectiveState(done, new Map(), new Set(), now).tone).toBe("done");
    expect(effectiveState(after, new Map(), new Set(), now).tone).toBe("done");
  });

  it("maps status 4 to closed", () => {
    expect(effectiveState(closed, new Map(), new Set(), now)).toMatchObject({
      label: "Annullata",
      tone: "closed",
    });
  });

  it("maps an unknown status to closed", () => {
    const row = fseRow("0900A0000000013", "visit", 99, "2026-11-01T09:00:00Z");
    expect(effectiveState(row, new Map(), new Set(), now).tone).toBe("closed");
  });
});

function expiringRow(nre: string, status: number, expiresOn: string): RowBody {
  const row = fseRow(nre, "visit", status, "2026-11-01T09:00:00Z");
  return row.fse === undefined ? row : { nre, fse: { ...row.fse, expiresOn } };
}

describe("effectiveState expiry", () => {
  const yesterday = "2026-11-09T09:00:00Z";
  const today = "2026-11-10T20:00:00Z";
  const future = "2026-12-01T09:00:00Z";

  it("is expired when expiresOn is yesterday", () => {
    const row = expiringRow("0900A0000000050", 3, yesterday);
    expect(effectiveState(row, new Map(), new Set(), now)).toEqual({
      kind: "expired",
      label: "Scaduta",
      tone: "closed",
      expiresOn: yesterday,
      estimated: false,
    });
  });

  it("flags an estimated expiry", () => {
    const row = expiringRow("0900A0000000050", 3, yesterday);
    const estimated: RowBody =
      row.fse === undefined
        ? row
        : { nre: row.nre, fse: { ...row.fse, expiryEstimated: true } };
    expect(effectiveState(estimated, new Map(), new Set(), now)).toMatchObject({
      kind: "expired",
      estimated: true,
    });
  });

  it("does not expire an FSE status 30 row that is unbooked in the CUP", () => {
    const row = expiringRow("0900A0000000058", 30, yesterday);
    expect(
      effectiveState(row, new Map(), new Set(["0900A0000000058"]), now).kind,
    ).toBe("official");
  });

  it("is not expired when expiresOn is today", () => {
    const row = expiringRow("0900A0000000051", 3, today);
    expect(effectiveState(row, new Map(), new Set(), now).kind).toBe(
      "official",
    );
  });

  it("is not expired when expiresOn is in the future", () => {
    const row = expiringRow("0900A0000000052", 3, future);
    expect(effectiveState(row, new Map(), new Set(), now).kind).toBe(
      "official",
    );
  });

  it("stays booked when an active booking exists", () => {
    const row = expiringRow("0900A0000000053", 3, yesterday);
    const bookings = new Map([
      [
        "0900A0000000053",
        booking("0900A0000000053", "PRENOTATO", "2026-12-01T09:00:00Z"),
      ],
    ]);
    expect(effectiveState(row, bookings, new Set(), now).kind).toBe("booked");
  });

  it("stays official for a status 8 row", () => {
    const row = expiringRow("0900A0000000055", 8, yesterday);
    expect(effectiveState(row, new Map(), new Set(), now)).toMatchObject({
      kind: "official",
      tone: "done",
    });
  });
});

describe("expired rows in filters and counts", () => {
  const expired = expiringRow("0900A0000000056", 3, "2026-11-09T09:00:00Z");
  const rows = [toBook, expired];

  it("Tutte hides expired rows when showExpired is false", () => {
    expect(filterRows(rows, base, empty)).toEqual([toBook]);
  });

  it("Tutte shows expired rows when showExpired is true", () => {
    expect(filterRows(rows, { ...base, showExpired: true }, empty)).toEqual(
      rows,
    );
  });

  it("Da prenotare never shows expired rows", () => {
    const criteria: Criteria = { ...base, status: "bookable" };
    expect(filterRows(rows, criteria, empty)).toEqual([toBook]);
    expect(filterRows(rows, { ...criteria, showExpired: true }, empty)).toEqual(
      [toBook],
    );
  });

  it("Aperte never shows expired rows", () => {
    const criteria: Criteria = { ...base, status: "open" };
    expect(filterRows(rows, criteria, empty)).toEqual([toBook]);
    expect(filterRows(rows, { ...criteria, showExpired: true }, empty)).toEqual(
      [toBook],
    );
  });

  it("counts hide expired rows when showExpired is false", () => {
    expect(countByStatus(rows, "", false, empty)).toEqual({
      upcoming: 0,
      bookable: 1,
      open: 1,
      all: 1,
    });
  });

  it("counts include expired rows in Tutte when showExpired is true", () => {
    expect(countByStatus(rows, "", true, empty)).toEqual({
      upcoming: 0,
      bookable: 1,
      open: 1,
      all: 2,
    });
  });
});

describe("unbooked FSE status 30 rows in counts", () => {
  const row = fseRow("0900A0000000060", "visit", 30, "2026-11-01T09:00:00Z");
  const context = contextOf([], ["0900A0000000060"]);

  it("counts as open but not bookable", () => {
    expect(countByStatus([row], "", false, context)).toMatchObject({
      bookable: 0,
      open: 1,
    });
  });

  it("is not bookable when the CUP lookup did not confirm it", () => {
    expect(countByStatus([row], "", false, empty)).toMatchObject({
      bookable: 0,
    });
  });
});

describe("countExpired", () => {
  const expired = expiringRow("0900A0000000057", 3, "2026-11-09T09:00:00Z");
  const rows = [toBook, expired, cupOnly];

  it("counts expired rows", () => {
    expect(countExpired(rows, "", empty)).toBe(1);
  });

  it("applies the search", () => {
    expect(countExpired(rows, "ecografia", empty)).toBe(0);
    expect(countExpired(rows, "0900A0000000057", empty)).toBe(1);
  });
});

describe("buildRows", () => {
  it("excludes drug prescriptions", () => {
    expect(buildRows([toBook, drug], [])).toEqual([toBook]);
  });

  it("keeps cup only rows", () => {
    expect(buildRows([cupOnly], [])).toEqual([cupOnly]);
  });

  it("adds a row for an appointment without a prescription row", () => {
    const appointment = booking("0900A0000000020", "PRENOTATO", undefined);
    expect(buildRows([toBook], [appointment])).toEqual([
      toBook,
      { nre: "0900A0000000020" },
    ]);
  });

  it("carries the appointment fse and cup into the added row", () => {
    const withFse = fseRow(
      "0900A0000000021",
      "visit",
      30,
      "2026-11-01T09:00:00Z",
    );
    const appointment: AppointmentBody = {
      ...booking("0900A0000000021", "PRENOTATO", undefined),
      ...(withFse.fse === undefined ? {} : { fse: withFse.fse }),
    };
    const rows = buildRows([], [appointment]);
    expect(rows.map((row) => rowTitle(row))).toEqual(["Visita esempio"]);
  });

  it("does not duplicate an appointment whose NRE already has a row", () => {
    const appointment = booking("0900a0000000001", "PRENOTATO", undefined);
    expect(buildRows([toBook], [appointment])).toEqual([toBook]);
  });

  it("adds one row for several appointments with the same NRE", () => {
    const first = booking("0900A0000000022", "PRENOTATO", undefined);
    const second = booking("0900A0000000022", "DISDETTO", undefined);
    expect(buildRows([], [first, second])).toHaveLength(1);
  });
});

describe("filterRows", () => {
  const rows = [toBook, pending, done, closed, cupOnly];
  const context = contextOf([
    booking("0900A0000000002", "PRENOTATO", "2026-11-10T14:00:00Z"),
    booking("0900A0000000003", "PRENOTATO", "2026-11-12T14:00:00Z"),
    booking("0900A0000000004", "PRENOTATO", "2026-11-01T14:00:00Z"),
  ]);

  it("upcoming keeps booked rows with an at from today", () => {
    const got = filterRows(rows, { ...base, status: "upcoming" }, context);
    expect(got).toEqual([pending, done]);
  });

  it("upcoming counts an appointment earlier today", () => {
    const earlier = contextOf([
      booking("0900A0000000002", "PRENOTATO", "2026-11-10T05:00:00Z"),
    ]);
    const got = filterRows([pending], { ...base, status: "upcoming" }, earlier);
    expect(got).toEqual([pending]);
  });

  it("bookable keeps official status 3 only", () => {
    const got = filterRows(rows, { ...base, status: "bookable" }, context);
    expect(got).toEqual([toBook, cupOnly]);
  });

  it("open keeps open rows and future booked rows, even if done", () => {
    const got = filterRows(rows, { ...base, status: "open" }, context);
    expect(got).toEqual([toBook, pending, done, cupOnly]);
  });

  it("open excludes a past booked closed row", () => {
    const got = filterRows([closed], { ...base, status: "open" }, context);
    expect(got).toEqual([]);
  });

  it("all keeps everything", () => {
    expect(filterRows(rows, base, context)).toEqual(rows);
  });

  it("searches the title case-insensitively", () => {
    const got = filterRows(rows, { ...base, search: "ECOGRAFIA" }, empty);
    expect(got).toEqual([cupOnly]);
  });

  it("searches the NRE", () => {
    const got = filterRows(rows, { ...base, search: "0900a0000000002" }, empty);
    expect(got).toEqual([pending]);
  });

  it("searches the unit name and city", () => {
    const withCity = contextOf([
      booking("0900A0000000002", "PRENOTATO", undefined, "Pisa"),
    ]);
    expect(filterRows(rows, { ...base, search: "pisa" }, withCity)).toEqual([
      pending,
    ]);
    expect(filterRows(rows, { ...base, search: "UNITA" }, withCity)).toEqual([
      pending,
    ]);
  });

  it("ignores a blank search", () => {
    expect(filterRows(rows, { ...base, search: "   " }, empty)).toEqual(rows);
  });

  it("handles null cup items", () => {
    expect(filterRows([nullItems], { ...base, search: "x" }, empty)).toEqual(
      [],
    );
  });
});

describe("countByStatus", () => {
  const rows = [toBook, pending, done, cupOnly];
  const context = contextOf([
    booking("0900A0000000002", "PRENOTATO", "2026-11-12T14:00:00Z"),
  ]);

  it("counts each status filter", () => {
    expect(countByStatus(rows, "", false, context)).toEqual({
      upcoming: 1,
      bookable: 2,
      open: 3,
      all: 4,
    });
  });

  it("applies the search before counting", () => {
    expect(countByStatus(rows, "ecografia", false, context)).toEqual({
      upcoming: 0,
      bookable: 1,
      open: 1,
      all: 1,
    });
  });
});

describe("sortRows default appuntamento", () => {
  const soon = fseRow("0900A0000000030", "visit", 30, "2026-10-01T09:00:00Z");
  const later = fseRow("0900A0000000031", "visit", 30, "2026-10-02T09:00:00Z");
  const past = fseRow("0900A0000000032", "visit", 30, "2026-10-20T09:00:00Z");
  const context = contextOf([
    booking("0900A0000000030", "PRENOTATO", "2026-11-11T09:00:00Z"),
    booking("0900A0000000031", "PRENOTATO", "2026-12-01T09:00:00Z"),
    booking("0900A0000000032", "PRENOTATO", "2026-11-01T09:00:00Z"),
  ]);
  const rows = [toBook, past, later, pending, soon];

  it("puts upcoming rows first, soonest first, then the rest by date desc", () => {
    expect(sortRows(rows, "appuntamento", "asc", context)).toEqual([
      soon,
      later,
      toBook,
      pending,
      past,
    ]);
  });

  it("flips only the dated group on desc", () => {
    expect(sortRows(rows, "appuntamento", "desc", context)).toEqual([
      later,
      soon,
      toBook,
      pending,
      past,
    ]);
  });

  it("does not mutate the input", () => {
    const input = [pending, toBook];
    sortRows(input, "appuntamento", "asc", empty);
    expect(input).toEqual([pending, toBook]);
  });

  it("orders undated rows by prescription date, undated last", () => {
    expect(
      sortRows([nullItems, pending, cupOnly], "appuntamento", "asc", empty),
    ).toEqual([cupOnly, pending, nullItems]);
  });
});

describe("sortRows by column", () => {
  const a = fseRow(
    "0900A0000000040",
    "visit",
    3,
    "2026-11-01T09:00:00Z",
    "Alfa",
  );
  const b = fseRow(
    "0900A0000000041",
    "visit",
    3,
    "2026-11-03T09:00:00Z",
    "Alfa",
  );
  const c = fseRow(
    "0900A0000000042",
    "visit",
    5,
    "2026-11-02T09:00:00Z",
    "Beta",
  );

  it("sorts by prestazione with the date desc tie-break", () => {
    expect(sortRows([c, a, b], "prestazione", "asc", empty)).toEqual([b, a, c]);
  });

  it("reverses the primary column but keeps the tie-break", () => {
    expect(sortRows([a, c, b], "prestazione", "desc", empty)).toEqual([
      c,
      b,
      a,
    ]);
  });

  it("sorts by prescription date", () => {
    expect(sortRows([a, b, c], "prescritta", "desc", empty)).toEqual([b, c, a]);
    expect(sortRows([a, b, c], "prescritta", "asc", empty)).toEqual([a, c, b]);
  });

  it("sorts by NRE", () => {
    expect(sortRows([c, a, b], "nre", "asc", empty)).toEqual([a, b, c]);
  });

  it("sorts by stato label with tie-break", () => {
    expect(sortRows([c, a, b], "stato", "asc", empty)).toEqual([b, a, c]);
  });

  it("sorts by struttura, rows without a booking last", () => {
    const context = contextOf([
      { ...booking("0900A0000000042", "PRENOTATO", undefined) },
    ]);
    expect(sortRows([a, c, b], "struttura", "asc", context)).toEqual([c, b, a]);
    expect(sortRows([a, c, b], "struttura", "desc", context)).toEqual([
      c,
      b,
      a,
    ]);
  });
});

describe("rowDate", () => {
  it("prefers the fse date over the cup date", () => {
    const row: RowBody = {
      ...toBook,
      cup: {
        nre: toBook.nre,
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
    expect(rowTitle(toBook)).toBe("Visita esempio");
  });

  it("joins the cup items", () => {
    expect(rowTitle(cupOnly)).toBe("Ecografia esempio, Esame sangue");
  });

  it("falls back to a generic title for null items", () => {
    expect(rowTitle(nullItems)).toBe("Prescrizione");
  });
});

describe("unitName", () => {
  const unit = {
    name: "Nome",
    description: "",
    address: "",
    city: "",
    province: "",
  };

  it("prefers the description", () => {
    expect(unitName({ ...unit, description: "Descrizione" })).toBe(
      "Descrizione",
    );
  });

  it("falls back to the name", () => {
    expect(unitName(unit)).toBe("Nome");
  });
});
