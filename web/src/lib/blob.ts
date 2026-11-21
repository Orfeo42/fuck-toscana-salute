const readFailed = "Impossibile leggere il file";

export function blobToDataUrl(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.addEventListener("load", () => {
      if (typeof reader.result === "string") {
        resolve(reader.result);
        return;
      }
      reject(new Error(readFailed));
    });
    reader.addEventListener("error", () => {
      reject(new Error(readFailed));
    });
    reader.readAsDataURL(blob);
  });
}
