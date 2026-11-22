import createClient from "openapi-fetch";

import type { components, paths } from "./schema";

export const api = createClient<paths>({ baseUrl: "" });

export type SessionBody = components["schemas"]["SessionBody"];
export type PortalBody = components["schemas"]["PortalBody"];
export type GoogleCodeRequest = components["schemas"]["GoogleCodeRequest"];
export type GoogleStatusBody = components["schemas"]["GoogleStatusBody"];
export type CalendarEventBody = components["schemas"]["CalendarEventBody"];
export type ReportBody = components["schemas"]["ReportBody"];
export type ReportFailureBody = components["schemas"]["ReportFailureBody"];
export type ReportDocumentBody = components["schemas"]["ReportDocumentBody"];
export type ReportRefRequest = components["schemas"]["ReportRefRequest"];
export type ReportPDFRequest = components["schemas"]["ReportPDFRequest"];
export type GoogleSettingsBody = components["schemas"]["GoogleSettingsBody"];
export type GoogleSettingsRequest =
  components["schemas"]["GoogleSettingsRequest"];
export type ReportRefBody = components["schemas"]["ReportRefBody"];
export type LinkRequest = components["schemas"]["LinkRequest"];
export type VisitStatusRequest = components["schemas"]["VisitStatusRequest"];
export type VisitNRERequest = components["schemas"]["VisitNRERequest"];
export type ImportDriveRequest = components["schemas"]["ImportDriveRequest"];
export type UpdateLocalReportRequest =
  components["schemas"]["UpdateLocalReportRequest"];
export type LocalReportIDRequest =
  components["schemas"]["LocalReportIDRequest"];
export type PickerBody = components["schemas"]["PickerBody"];
export type StatusOptionBody = components["schemas"]["StatusOptionBody"];
export type UpdatedBody = components["schemas"]["UpdatedBody"];
export type FocusBody = components["schemas"]["FocusBody"];
export type RowRequest = components["schemas"]["RowRequest"];
export type VisitsQueryRequest = components["schemas"]["VisitsQueryRequest"];
export type VisitsQueryBody = components["schemas"]["VisitsQueryBody"];
export type VisitsIssuesBody = components["schemas"]["VisitsIssuesBody"];
export type VisitRowBody = components["schemas"]["VisitRowBody"];
export type VisitStatusBody = components["schemas"]["VisitStatusBody"];
export type VisitAppointmentBody =
  components["schemas"]["VisitAppointmentBody"];
export type VisitUnitBody = components["schemas"]["VisitUnitBody"];
export type CalendarFieldsBody = components["schemas"]["CalendarFieldsBody"];
export type LinkedReportBody = components["schemas"]["LinkedReportBody"];
export type VisitCheckedBody = components["schemas"]["VisitCheckedBody"];
export type UploadDefaultsBody = components["schemas"]["UploadDefaultsBody"];
export type VisitRowResponseBody =
  components["schemas"]["VisitRowResponseBody"];
export type ManualStatusBody = components["schemas"]["ManualStatusBody"];
export type ManualStatusOptionBody =
  components["schemas"]["ManualStatusOptionBody"];
export type VisitReportOptionsBody =
  components["schemas"]["VisitReportOptionsBody"];
export type ReportOptionBody = components["schemas"]["ReportOptionBody"];
export type ReportsQueryRequest = components["schemas"]["ReportsQueryRequest"];
export type ReportsQueryBody = components["schemas"]["ReportsQueryBody"];
export type KindOptionBody = components["schemas"]["KindOptionBody"];
export type ReportRowBody = components["schemas"]["ReportRowBody"];
export type LinkedVisitBody = components["schemas"]["LinkedVisitBody"];
export type ReportRowRequest = components["schemas"]["ReportRowRequest"];
export type ReportRowResponseBody =
  components["schemas"]["ReportRowResponseBody"];
export type ReportVisitOptionsBody =
  components["schemas"]["ReportVisitOptionsBody"];
export type DrugsQueryRequest = components["schemas"]["DrugsQueryRequest"];
export type DrugsQueryBody = components["schemas"]["DrugsQueryBody"];
export type DrugRowBody = components["schemas"]["DrugRowBody"];
export type DrugStatusBody = components["schemas"]["DrugStatusBody"];
export type DrugRowResponseBody = components["schemas"]["DrugRowResponseBody"];
export type StartRefreshRequest = components["schemas"]["StartRefreshRequest"];
export type StartRefreshBody = components["schemas"]["StartRefreshBody"];

export function prescriptionPdfUrl(ref: string): string {
  const query = new URLSearchParams({ ref });
  return `/api/documents/prescription?${query.toString()}`;
}

export function bookingPdfUrl(bookingId: string): string {
  return `/api/documents/booking?id=${encodeURIComponent(bookingId)}`;
}
