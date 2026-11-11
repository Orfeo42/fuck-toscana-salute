import type { UnitBody } from "../../api/client";

export function fullAddress(unit: UnitBody): string {
  const locality = [unit.postalCode ?? "", unit.city, unit.province]
    .filter((part) => part !== "")
    .join(" ");
  return [unit.address, locality].filter((part) => part !== "").join(", ");
}

export function mapsDirectionsUrl(unit: UnitBody): string {
  return `https://www.google.com/maps/dir/?api=1&destination=${encodeURIComponent(fullAddress(unit))}`;
}

export function mapsSearchUrl(unit: UnitBody): string {
  const place = unit.place ?? "";
  const query =
    place === "" ? fullAddress(unit) : `${place}, ${fullAddress(unit)}`;
  return `https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(query)}`;
}
