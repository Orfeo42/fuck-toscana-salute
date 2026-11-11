import type { AppointmentBody } from "../../api/client";

const durationMs = 60 * 60 * 1000;

function utcBasic(date: Date): string {
  return date
    .toISOString()
    .replace(/\.\d{3}Z$/, "Z")
    .replace(/[-:]/g, "");
}

export function googleCalendarUrl(
  appointment: AppointmentBody,
): string | undefined {
  const { at, unit, fse } = appointment;
  if (at === undefined) {
    return undefined;
  }
  const start = new Date(at);
  const end = new Date(start.getTime() + durationMs);
  const place = [unit.address, unit.city, unit.province]
    .filter((part) => part !== "")
    .join(", ");
  const params = new URLSearchParams({
    action: "TEMPLATE",
    text:
      fse !== undefined && fse.description !== ""
        ? fse.description
        : "Visita / esame",
    dates: `${utcBasic(start)}/${utcBasic(end)}`,
    location: [unit.description !== "" ? unit.description : unit.name, place]
      .filter((part) => part !== "")
      .join(", "),
    details: "Prenotazione CUP Regione Toscana",
  });
  return `https://calendar.google.com/calendar/render?${params.toString()}`;
}
