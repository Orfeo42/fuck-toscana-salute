export type PortalId = "cup" | "fse";

export interface TokensMessage {
  access: string;
  refresh: string;
  cf: string;
  tessera: string;
}

const scheme = "javascript:";
const portalOrigins = new Map<string, PortalId>([
  ["https://prenota.sanita.toscana.it", "cup"],
  ["https://fse.sanita.toscana.it", "fse"],
]);

export const portalUrls: Readonly<Record<PortalId, string>> = {
  cup: "https://prenota.sanita.toscana.it/",
  fse: "https://fse.sanita.toscana.it/",
};

const missingTokensAlert =
  "Accedi prima al portale, poi tocca di nuovo il segnalibro.";
const popupBlockedAlert =
  "Impossibile aprire Toscana Salute: consenti i popup e riprova.";

export function bookmarkletCode(appOrigin: string): string {
  const origin = JSON.stringify(appOrigin);
  const body = [
    "var s=sessionStorage,a=s.getItem('access_token'),r=s.getItem('refresh_token'),c=s.getItem('fcode')||s.getItem('fseCf')||'',t='';",
    `if(!a||!r){alert(${JSON.stringify(missingTokensAlert)});return}`,
    "var e=performance.getEntriesByType('resource');",
    "for(var i=e.length-1;i>=0;i--){var n=e[i];if(n.name.indexOf('/verificaTs?')>-1&&(!n.responseStatus||n.responseStatus===200)){try{t=new URL(n.name).searchParams.get('numbersTeam')||''}catch(x){}break}}",
    `var o=${origin},w=window.open(o+'/\\x23/collega','fts-collega');`,
    `if(!w){alert(${JSON.stringify(popupBlockedAlert)});return}`,
    "function h(v){if(v.origin!==o||v.source!==w||!v.data||v.data.type!=='fts-ready')return;window.removeEventListener('message',h);w.postMessage({type:'fts-tokens',access:a,refresh:r,cf:c,tessera:t},o)}",
    "window.addEventListener('message',h);",
  ].join("");
  return `${scheme}void(function(){${body}}())`;
}

export function portalForOrigin(origin: string): PortalId | undefined {
  return portalOrigins.get(origin);
}

type RawMessage = Partial<
  Record<"type" | "access" | "refresh" | "cf" | "tessera", unknown>
>;

function isRecord(value: unknown): value is RawMessage {
  return typeof value === "object" && value !== null;
}

export function parseTokensMessage(data: unknown): TokensMessage | undefined {
  if (!isRecord(data) || data.type !== "fts-tokens") {
    return undefined;
  }
  const { access, refresh, cf, tessera } = data;
  if (
    typeof access !== "string" ||
    typeof refresh !== "string" ||
    typeof cf !== "string" ||
    typeof tessera !== "string"
  ) {
    return undefined;
  }
  return { access, refresh, cf, tessera };
}
