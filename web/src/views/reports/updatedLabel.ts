const dayFormat = new Intl.DateTimeFormat("it-IT", {
  timeZone: "Europe/Rome",
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
});

const dayMonthFormat = new Intl.DateTimeFormat("it-IT", {
  timeZone: "Europe/Rome",
  day: "2-digit",
  month: "2-digit",
});

const timeFormat = new Intl.DateTimeFormat("it-IT", {
  timeZone: "Europe/Rome",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});

export function updatedLabel(iso: string, now: Date): string {
  const date = new Date(iso);
  const time = timeFormat.format(date);
  if (dayFormat.format(date) === dayFormat.format(now)) {
    return `Aggiornato alle ${time}`;
  }
  return `Aggiornato il ${dayMonthFormat.format(date)} ${time}`;
}
