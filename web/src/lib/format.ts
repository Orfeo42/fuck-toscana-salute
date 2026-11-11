const dateFormat = new Intl.DateTimeFormat("it-IT", {
  timeZone: "Europe/Rome",
  weekday: "short",
  day: "numeric",
  month: "short",
  year: "numeric",
});

const dateTimeFormat = new Intl.DateTimeFormat("it-IT", {
  timeZone: "Europe/Rome",
  weekday: "short",
  day: "numeric",
  month: "short",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});

const timeFormat = new Intl.DateTimeFormat("it-IT", {
  timeZone: "Europe/Rome",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});

const missing = "—";

export function formatDate(iso: string | undefined): string {
  if (iso === undefined) {
    return missing;
  }
  return dateFormat.format(new Date(iso));
}

export function formatDateTime(iso: string | undefined): string {
  if (iso === undefined) {
    return missing;
  }
  return dateTimeFormat.format(new Date(iso));
}

export function formatTime(iso: string | undefined): string {
  if (iso === undefined) {
    return missing;
  }
  return timeFormat.format(new Date(iso));
}

const compactDateFormat = new Intl.DateTimeFormat("it-IT", {
  timeZone: "Europe/Rome",
  weekday: "short",
  day: "2-digit",
  month: "2-digit",
});

const longDateFormat = new Intl.DateTimeFormat("it-IT", {
  timeZone: "Europe/Rome",
  weekday: "long",
  day: "numeric",
  month: "long",
  year: "numeric",
});

export function formatCompactDateTime(iso: string): string {
  const date = new Date(iso);
  return `${compactDateFormat.format(date)} · ${timeFormat.format(date)}`;
}

export function formatLongDateTime(iso: string): string {
  const date = new Date(iso);
  return `${longDateFormat.format(date)}, ${timeFormat.format(date)}`;
}
