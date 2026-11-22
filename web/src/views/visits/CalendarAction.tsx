import { CalendarCheck } from "lucide-react";
import { useState, type JSX } from "react";

import type { VisitAppointmentBody } from "../../api/client";
import { IconAction } from "../../components/IconAction";
import { CalendarDialog } from "./CalendarDialog";

interface Props {
  appointment: VisitAppointmentBody;
}

export function CalendarAction({ appointment }: Props): JSX.Element {
  const [open, setOpen] = useState(false);
  return (
    <>
      <IconAction
        label="Aggiungi a Google Calendar"
        icon={CalendarCheck}
        onClick={() => {
          setOpen(true);
        }}
      />
      {open ? (
        <CalendarDialog
          appointment={appointment}
          onClose={() => {
            setOpen(false);
          }}
        />
      ) : null}
    </>
  );
}
