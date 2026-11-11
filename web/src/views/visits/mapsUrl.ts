function destination(address: string, city: string): string {
  return encodeURIComponent(`${address}, ${city}`);
}

export function mapsDirectionsUrl(address: string, city: string): string {
  return `https://www.google.com/maps/dir/?api=1&destination=${destination(address, city)}`;
}

export function mapsSearchUrl(address: string, city: string): string {
  return `https://www.google.com/maps/search/?api=1&query=${destination(address, city)}`;
}
