const prescriptionLabels: Readonly<Record<number, string>> = {
  3: "Da prenotare",
  4: "Annullata",
  5: "Presa in carico",
  6: "Sospesa",
  7: "Erogata in parte",
  8: "Erogata",
  9: "Erogata dopo annullamento",
  30: "Prenotata",
};

const closedStatuses: ReadonlySet<number> = new Set([4, 8, 9]);

const failureLabels: Readonly<Record<string, string>> = {
  not_in_cup:
    "Prenotazione non presente nel CUP regionale (forse fatta per telefono o allo sportello)",
};

export function prescriptionStatusLabel(status: number): string {
  return prescriptionLabels[status] ?? `Stato ${String(status)}`;
}

export function isClosedPrescription(status: number): boolean {
  return closedStatuses.has(status);
}

export function appointmentStatusLabel(status: string): string {
  const lower = status.toLowerCase();
  return lower.charAt(0).toUpperCase() + lower.slice(1);
}

export function failureLabel(reason: string): string {
  return failureLabels[reason] ?? "Errore del portale regionale";
}
