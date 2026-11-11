import type { JSX } from "react";

import { prescriptionPdfUrl } from "../api/client";

interface Props {
  documentRef: string | undefined;
  label?: string;
}

export function PdfLink({
  documentRef,
  label = "Apri ricetta (PDF)",
}: Props): JSX.Element | null {
  if (documentRef === undefined || documentRef === "") {
    return null;
  }
  return (
    <a
      className="pdf-link"
      href={prescriptionPdfUrl(documentRef)}
      target="_blank"
      rel="noopener noreferrer"
    >
      {label}
    </a>
  );
}
