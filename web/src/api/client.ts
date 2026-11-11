import createClient from "openapi-fetch";

import type { components, paths } from "./schema";

export const api = createClient<paths>({ baseUrl: "" });

export type SessionBody = components["schemas"]["SessionBody"];
export type PortalBody = components["schemas"]["PortalBody"];
export type AppointmentsBody = components["schemas"]["AppointmentsBody"];
export type AppointmentBody = components["schemas"]["AppointmentBody"];
export type FailedBody = components["schemas"]["FailedBody"];
export type PrescriptionsBody = components["schemas"]["PrescriptionsBody"];
export type RowBody = components["schemas"]["RowBody"];
export type PrescriptionCUPBody = components["schemas"]["PrescriptionCUPBody"];
export type PrescriptionFSEBody = components["schemas"]["PrescriptionFSEBody"];
export type UnitBody = components["schemas"]["UnitBody"];

export function prescriptionPdfUrl(ref: string): string {
  const query = new URLSearchParams({ ref });
  return `/api/documents/prescription?${query.toString()}`;
}

export function bookingPdfUrl(bookingId: string): string {
  return `/api/documents/booking?id=${encodeURIComponent(bookingId)}`;
}
