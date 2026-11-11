const dayFormat = new Intl.DateTimeFormat("en-CA", {
  timeZone: "Europe/Rome",
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
});

const msPerDay = 86_400_000;

function dayNumber(date: Date): number {
  const [year, month, day] = dayFormat
    .format(date)
    .split("-")
    .map((part) => Number(part));
  return Date.UTC(year ?? 0, (month ?? 1) - 1, day ?? 1) / msPerDay;
}

export function relativeDay(iso: string, now: Date): string {
  const days = dayNumber(new Date(iso)) - dayNumber(now);
  if (days === 0) {
    return "oggi";
  }
  if (days === 1) {
    return "domani";
  }
  if (days === -1) {
    return "ieri";
  }
  if (days < 0) {
    return `${String(-days)} giorni fa`;
  }
  return `tra ${String(days)} giorni`;
}

export function romeDay(date: Date): string {
  return dayFormat.format(date);
}
