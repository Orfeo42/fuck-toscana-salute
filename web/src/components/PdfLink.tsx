import { FileText } from "lucide-react";
import { useState } from "react";
import type { JSX } from "react";

import { prescriptionPdfUrl } from "../api/client";
import { IconAction } from "./IconAction";
import { PdfViewerModal } from "./PdfViewerModal";

interface Props {
  documentRef: string | undefined;
  label?: string;
  labelled?: boolean;
}

export function PdfLink({
  documentRef,
  label = "Apri ricetta (PDF)",
  labelled = false,
}: Props): JSX.Element | null {
  const [opened, setOpened] = useState(false);
  if (documentRef === undefined || documentRef === "") {
    return null;
  }
  return (
    <>
      <IconAction
        label={label}
        icon={FileText}
        labelled={labelled}
        onClick={() => {
          setOpened(true);
        }}
      />
      <PdfViewerModal
        url={prescriptionPdfUrl(documentRef)}
        title="Ricetta"
        opened={opened}
        onClose={() => {
          setOpened(false);
        }}
      />
    </>
  );
}
