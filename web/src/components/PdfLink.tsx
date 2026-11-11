import { FileText } from "lucide-react";
import type { JSX } from "react";

import { prescriptionPdfUrl } from "../api/client";
import { IconAction } from "./IconAction";

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
    <IconAction
      label={label}
      icon={FileText}
      href={prescriptionPdfUrl(documentRef)}
      external
    />
  );
}
