import { Anchor, Group, Select, Stack, Text } from "@mantine/core";
import { SquareArrowOutUpRight, Unlink } from "lucide-react";
import { useState } from "react";
import type { JSX } from "react";

import { api, type ReportRefBody } from "../../api/client";
import { CappedList } from "../../components/CappedList";
import { IconAction } from "../../components/IconAction";
import { useHealthData } from "../../data/useHealthData";
import { requestOpenVisit } from "../../lib/openVisit";
import { buildRows, rowTitle } from "../visits/filter";

interface Props {
  reportRef: ReportRefBody;
  nres: readonly string[];
  onLinked: (nre: string) => void;
  onUnlinked: (nre: string) => void;
}

const linkFailed = "Operazione non riuscita";

export function VisitLinks({
  reportRef,
  nres,
  onLinked,
  onUnlinked,
}: Props): JSX.Element {
  const { prescriptions, appointments } = useHealthData();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | undefined>(undefined);

  const rows = buildRows(
    prescriptions.data?.prescriptions ?? [],
    appointments.data?.appointments ?? [],
  );
  const linkedKeys = nres.map((nre) => nre.toUpperCase());
  const linked = nres.map((nre) => ({
    nre,
    row: rows.find((row) => row.nre.toUpperCase() === nre.toUpperCase()),
  }));
  const options = rows
    .filter((row) => !linkedKeys.includes(row.nre.toUpperCase()))
    .map((row) => ({
      value: row.nre,
      label: `${rowTitle(row)} · ${row.nre}`,
    }));

  const change = async (nre: string, link: boolean): Promise<void> => {
    setBusy(true);
    setError(undefined);
    try {
      const body = { nre, report: reportRef };
      const { response } = link
        ? await api.PUT("/api/links", { body })
        : await api.DELETE("/api/links", { body });
      if (!response.ok) {
        setError(linkFailed);
        return;
      }
      if (link) {
        onLinked(nre);
        return;
      }
      onUnlinked(nre);
    } catch {
      setError(linkFailed);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Stack gap="xs">
      {linked.length === 0 ? <Text>Nessuna visita collegata.</Text> : null}
      <CappedList
        items={linked}
        getKey={({ nre }) => nre}
        renderItem={({ nre, row }) => (
          <Group justify="space-between" wrap="nowrap">
            <Stack gap={0}>
              <Anchor
                component="button"
                type="button"
                ta="left"
                onClick={() => {
                  requestOpenVisit(nre);
                }}
              >
                {row === undefined ? nre : rowTitle(row)}
              </Anchor>
              <Text size="xs" c="dimmed">
                {nre}
              </Text>
            </Stack>
            <Group gap="xs" wrap="nowrap">
              <IconAction
                label="Apri visita"
                icon={SquareArrowOutUpRight}
                onClick={() => {
                  requestOpenVisit(nre);
                }}
              />
              <IconAction
                label={`Scollega ${nre}`}
                icon={Unlink}
                onClick={() => {
                  void change(nre, false);
                }}
              />
            </Group>
          </Group>
        )}
      />
      <Select
        label="Collega a visita"
        placeholder="Cerca una visita"
        searchable
        nothingFoundMessage="Nessuna visita trovata"
        data={options}
        value={null}
        disabled={busy}
        onChange={(nre) => {
          if (nre !== null) {
            void change(nre, true);
          }
        }}
      />
      {error === undefined ? null : (
        <Text size="sm" c="red" role="alert">
          {error}
        </Text>
      )}
    </Stack>
  );
}
