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

const shortDateFormat = new Intl.DateTimeFormat("it-IT", {
  timeZone: "Europe/Rome",
  day: "2-digit",
  month: "2-digit",
});

const dayKeyFormat = new Intl.DateTimeFormat("en-CA", {
  timeZone: "Europe/Rome",
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
});

export function formatShortDateTime(iso: string): string {
  const date = new Date(iso);
  return `${shortDateFormat.format(date)} ${timeFormat.format(date)}`;
}

export function formatUpdated(
  iso: string | null | undefined,
  now: Date,
): string | undefined {
  if (iso === null || iso === undefined) {
    return undefined;
  }
  const date = new Date(iso);
  if (dayKeyFormat.format(date) === dayKeyFormat.format(now)) {
    return `Aggiornato alle ${timeFormat.format(date)}`;
  }
  return `Aggiornato il ${formatShortDateTime(iso)}`;
}
