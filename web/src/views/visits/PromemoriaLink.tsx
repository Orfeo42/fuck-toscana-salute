import { Ticket } from "lucide-react";
import type { JSX } from "react";

import { bookingPdfUrl } from "../../api/client";
import { IconAction } from "../../components/IconAction";

interface Props {
  bookingId: string | undefined;
}

export function PromemoriaLink({ bookingId }: Props): JSX.Element | null {
  if (bookingId === undefined || bookingId === "") {
    return null;
  }
  return (
    <IconAction
      label="Apri promemoria prenotazione (PDF)"
      icon={Ticket}
      href={bookingPdfUrl(bookingId)}
      external
    />
  );
}
