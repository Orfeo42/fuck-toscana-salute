import type {
  ManualStatusBody,
  StatusOptionBody,
  VisitAppointmentBody,
  VisitRowBody,
  VisitRowResponseBody,
  VisitsQueryBody,
  VisitUnitBody,
} from "../api/client";

export const nre1 = "0900A0000000001";
export const nre2 = "0900A0000000002";
export const nre3 = "0900A0000000003";

export function unit(extra: Partial<VisitUnitBody> = {}): VisitUnitBody {
  return {
    name: "OSP ESEMPIO",
    description: "Ospedale Esempio",
    displayName: "Ospedale Esempio",
    place: "Piano 2",
    address: "Via Finta 1",
    postalCode: "50100",
    city: "Firenze",
    province: "FI",
    fullAddress: "Via Finta 1, 50100 Firenze, FI",
    mapsSearchUrl: "https://maps.example.com/search?q=finta",
    mapsDirectionsUrl: "https://maps.example.com/dir?q=finta",
    ...extra,
  };
}

export function appointment(
  extra: Partial<VisitAppointmentBody> = {},
): VisitAppointmentBody {
  return {
    id: "APP1",
    bookingId: "BOOK1",
    status: "PRENOTATO",
    at: "2026-12-01T09:30:00+01:00",
    relativeDay: "tra 3 giorni",
    upcoming: true,
    unit: unit(),
    calendar: {
      title: "Visita esempio",
      location: "Ospedale Esempio, Via Finta 1, Firenze, FI",
      details: "Prenotazione CUP Regione Toscana",
    },
    ...extra,
  };
}

export function visitRow(extra: Partial<VisitRowBody> = {}): VisitRowBody {
  return {
    nre: nre1,
    title: "Visita esempio",
    prescribedOn: "2026-11-04T09:00:00Z",
    documentRef: "ref-1",
    status: {
      key: "prescribed",
      label: "Da prenotare",
      tone: "tobook",
      kind: "official",
      manual: false,
      canBook: true,
      expiryEstimated: false,
      details: [],
    },
    appointmentLookup: "none",
    reports: [],
    checkedAt: { appointments: null, verdict: null },
    upload: { title: "Visita esempio", date: "2026-11-04", provider: "" },
    refreshing: false,
    ...extra,
  };
}

export function bookedRow(extra: Partial<VisitRowBody> = {}): VisitRowBody {
  return visitRow({
    nre: nre2,
    title: "Controllo esempio",
    status: {
      key: "booked",
      label: "Prenotata",
      tone: "booked",
      kind: "booked",
      manual: false,
      canBook: false,
      expiryEstimated: false,
      details: [],
    },
    appointment: appointment(),
    appointmentLookup: "done",
    ...extra,
  });
}

export const statuses: StatusOptionBody[] = [
  {
    key: "booked",
    label: "Prenotata",
    tone: "booked",
    count: 1,
    excluded: false,
  },
  {
    key: "prescribed",
    label: "Da prenotare",
    tone: "tobook",
    count: 1,
    excluded: false,
  },
  { key: "done", label: "Erogata", tone: "done", count: 1, excluded: true },
];

export function queryBody(
  rows: VisitRowBody[],
  extra: Partial<VisitsQueryBody> = {},
): VisitsQueryBody {
  return {
    rows,
    total: rows.length,
    page: 1,
    pageSize: 50,
    statuses,
    excludedStatuses: ["done"],
    search: "",
    refreshing: false,
    updated: { at: "2026-11-10T08:00:00Z", label: "Aggiornato alle 09:00" },
    issues: { lists: "none", appointments: "none" },
    ...extra,
  };
}

export const noManualStatus: ManualStatusBody = {
  current: null,
  options: [
    { code: 3, label: "Da prenotare" },
    { code: 8, label: "Erogata" },
  ],
};

export function rowResponse(
  row: VisitRowBody,
  extra: Partial<VisitRowResponseBody> = {},
): VisitRowResponseBody {
  return {
    row,
    manualStatus: noManualStatus,
    refreshing: false,
    issues: { lists: "none", appointments: "none" },
    ...extra,
  };
}
