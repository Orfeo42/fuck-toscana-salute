import { CalendarPlus } from "lucide-react";
import type { JSX } from "react";

import { IconAction } from "../../components/IconAction";

const cupUrl = "https://prenota.sanita.toscana.it/";

interface Props {
  labelled?: boolean;
}

export function BookAction({ labelled = false }: Props): JSX.Element {
  return (
    <IconAction
      label="Prenota sul CUP"
      icon={CalendarPlus}
      labelled={labelled}
      href={cupUrl}
      external
    />
  );
}
