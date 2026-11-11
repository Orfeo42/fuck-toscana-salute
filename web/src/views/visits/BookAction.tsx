import type { JSX } from "react";

const cupUrl = "https://prenota.sanita.toscana.it/";

export function BookAction(): JSX.Element {
  return (
    <a href={cupUrl} target="_blank" rel="noopener noreferrer">
      Prenota sul CUP
    </a>
  );
}
