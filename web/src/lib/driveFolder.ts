const folderPath = /\/folders\/([^/?#]+)/;
const idParam = /[?&]id=([^&#]+)/;

export function driveFolderId(input: string): string {
  const trimmed = input.trim();
  const fromPath = folderPath.exec(trimmed);
  if (fromPath?.[1] !== undefined) {
    return fromPath[1];
  }
  const fromParam = idParam.exec(trimmed);
  if (fromParam?.[1] !== undefined) {
    return fromParam[1];
  }
  return trimmed;
}
