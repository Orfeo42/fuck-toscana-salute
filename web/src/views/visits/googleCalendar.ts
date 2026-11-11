import type { AppointmentBody } from "../../api/client";
import { unitName } from "./filter";

export interface CalendarEventFields {
  title: string;
  location: string;
  details: string;
}

export function calendarEventFields(
  appointment: AppointmentBody,
  title: string,
): CalendarEventFields {
  const { unit } = appointment;
  const place = [unit.address, unit.city, unit.province]
    .filter((part) => part !== "")
    .join(", ");
  return {
    title: title !== "" ? title : "Visita / esame",
    location: [unitName(unit), place].filter((part) => part !== "").join(", "),
    details: "Prenotazione CUP Regione Toscana",
  };
}
