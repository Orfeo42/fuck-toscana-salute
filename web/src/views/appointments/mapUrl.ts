export function mapUrl(address: string, city: string): string {
  const query = encodeURIComponent(`${address} ${city}`);
  return `https://www.openstreetmap.org/search?query=${query}`;
}
