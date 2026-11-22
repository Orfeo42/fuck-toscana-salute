import { CalendarCheck } from "lucide-react";
import { useState, type JSX } from "react";

import type { AppointmentBody } from "../../api/client";
import { IconAction } from "../../components/IconAction";
import { CalendarDialog } from "./CalendarDialog";

interface Props {
  appointment: AppointmentBody;
  title: string;
}

export function CalendarAction({ appointment, title }: Props): JSX.Element {
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
          title={title}
          onClose={() => {
            setOpen(false);
          }}
        />
      ) : null}
    </>
  );
}
