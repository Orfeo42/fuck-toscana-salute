import { CalendarPlus } from "lucide-react";
import type { JSX } from "react";

import { IconAction } from "../../components/IconAction";

const cupUrl = "https://prenota.sanita.toscana.it/";

export function BookAction(): JSX.Element {
  return (
    <IconAction
      label="Prenota sul CUP"
      icon={CalendarPlus}
      href={cupUrl}
      external
    />
  );
}
