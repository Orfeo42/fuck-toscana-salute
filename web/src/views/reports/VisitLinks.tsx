import { Anchor, Group, Select, Stack, Text } from "@mantine/core";
import { SquareArrowOutUpRight, Unlink } from "lucide-react";
import { useEffect, useState } from "react";
import type { JSX } from "react";

import {
  api,
  type LinkedVisitBody,
  type ReportRefBody,
} from "../../api/client";
import { CappedList } from "../../components/CappedList";
import { IconAction } from "../../components/IconAction";
import { requestOpenVisit } from "../../lib/openVisit";

interface Props {
  reportRef: ReportRefBody;
  visits: readonly LinkedVisitBody[];
  onChanged: () => void;
}

const linkFailed = "Operazione non riuscita";

export function VisitLinks({
  reportRef,
  visits,
  onChanged,
}: Props): JSX.Element {
  const [options, setOptions] = useState<readonly LinkedVisitBody[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | undefined>(undefined);
  const linkedKey = visits.map((visit) => visit.nre).join("|");

  useEffect(() => {
    const controller = new AbortController();
    const run = async (): Promise<void> => {
      try {
        const { data } = await api.POST("/api/reports/visit-options", {
          body: { ref: reportRef },
          signal: controller.signal,
        });
        if (controller.signal.aborted) {
          return;
        }
        setOptions(data?.options ?? []);
      } catch {
        if (controller.signal.aborted) {
          return;
        }
        setOptions([]);
      }
    };
    void run();
    return () => {
      controller.abort();
    };
  }, [reportRef, linkedKey]);

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
      onChanged();
    } catch {
      setError(linkFailed);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Stack gap="xs">
      {visits.length === 0 ? <Text>Nessuna visita collegata.</Text> : null}
      <CappedList
        items={visits}
        getKey={({ nre }) => nre}
        renderItem={({ nre, title }) => (
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
                {title}
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
        data={options.map((option) => ({
          value: option.nre,
          label: `${option.title} · ${option.nre}`,
        }))}
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
