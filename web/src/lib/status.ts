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

export const prescriptionStatuses = [3, 30, 5, 6, 7, 8, 9, 4] as const;

export type SelectableStatus = (typeof prescriptionStatuses)[number];

const closedStatuses: ReadonlySet<number> = new Set([4, 8, 9]);

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
