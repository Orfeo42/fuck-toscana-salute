import { describe, expect, it } from "vitest";

import { relativeDay, romeDay } from "./relativeDay";

const now = new Date("2026-03-10T09:00:00+01:00");

describe("relativeDay", () => {
  it("says oggi for the same Rome day", () => {
    expect(relativeDay("2026-03-10T23:30:00+01:00", now)).toBe("oggi");
  });

  it("says domani for the next day", () => {
    expect(relativeDay("2026-03-11T08:00:00+01:00", now)).toBe("domani");
  });

  it("counts future days", () => {
    expect(relativeDay("2026-03-15T08:00:00+01:00", now)).toBe("tra 5 giorni");
  });

  it("says ieri for the previous day", () => {
    expect(relativeDay("2026-03-09T08:00:00+01:00", now)).toBe("ieri");
  });

  it("counts past days", () => {
    expect(relativeDay("2026-03-01T08:00:00+01:00", now)).toBe("9 giorni fa");
  });

  it("uses the Rome calendar day across midnight UTC", () => {
    expect(relativeDay("2026-03-10T23:30:00Z", now)).toBe("domani");
  });
});

describe("romeDay", () => {
  it("formats the Rome calendar day", () => {
    expect(romeDay(new Date("2026-03-10T23:30:00Z"))).toBe("2026-03-11");
  });
});
