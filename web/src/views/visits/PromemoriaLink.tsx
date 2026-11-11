import { Ticket } from "lucide-react";
import { useState } from "react";
import type { JSX } from "react";

import { bookingPdfUrl } from "../../api/client";
import { IconAction } from "../../components/IconAction";
import { PdfViewerModal } from "../../components/PdfViewerModal";

interface Props {
  bookingId: string | undefined;
  labelled?: boolean;
}

export function PromemoriaLink({
  bookingId,
  labelled = false,
}: Props): JSX.Element | null {
  const [opened, setOpened] = useState(false);
  if (bookingId === undefined || bookingId === "") {
    return null;
  }
  return (
    <>
      <IconAction
        label="Apri promemoria prenotazione (PDF)"
        icon={Ticket}
        labelled={labelled}
        onClick={() => {
          setOpened(true);
        }}
      />
      <PdfViewerModal
        url={bookingPdfUrl(bookingId)}
        title="Promemoria"
        opened={opened}
        onClose={() => {
          setOpened(false);
        }}
      />
    </>
  );
}
