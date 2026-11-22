import { Loader, Paper, Text } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { RefreshCw } from "lucide-react";
import { useState } from "react";
import type { JSX } from "react";

import { CopyButton } from "../../components/CopyButton";
import { DetailDrawer } from "../../components/DetailDrawer";
import { DetailField, DetailSection } from "../../components/DetailSection";
import { IconAction } from "../../components/IconAction";
import { PdfLink } from "../../components/PdfLink";
import { StatusBadge } from "../../components/StatusBadge";
import { startRefresh } from "../../data/refresh";
import { formatDate } from "../../lib/format";
import { useRowQuery } from "../../lib/useRowQuery";

interface Props {
  nre: string;
  onClose: () => void;
}

export function DrugDetailModal({ nre, onClose }: Props): JSX.Element {
  const query = useRowQuery("/api/drugs/row", { nre });
  const { data, reload } = query;
  const [starting, setStarting] = useState(false);

  if (data === undefined) {
    const message = query.notFound
      ? "Farmaco non trovato."
      : query.error === undefined
        ? undefined
        : "Impossibile caricare il farmaco.";
    return (
      <DetailDrawer
        title="Farmaco"
        tabs={[
          {
            value: "prescrizione",
            label: "Prescrizione",
            content:
              message === undefined ? (
                <Loader size="sm" aria-label="Carico il farmaco" />
              ) : (
                <Text size="sm">{message}</Text>
              ),
          },
        ]}
        onClose={onClose}
      />
    );
  }

  const { row } = data;

  const refreshLists = async (): Promise<void> => {
    setStarting(true);
    try {
      const ok = await startRefresh({ lists: true });
      if (!ok) {
        notifications.show({
          color: "red",
          message: "Aggiornamento non riuscito",
        });
        return;
      }
      reload();
    } catch {
      notifications.show({
        color: "red",
        message: "Aggiornamento non riuscito",
      });
    } finally {
      setStarting(false);
    }
  };

  return (
    <DetailDrawer
      title={row.title}
      badge={
        <StatusBadge tone={row.status.tone}>{row.status.label}</StatusBadge>
      }
      summary={
        row.date === undefined ? undefined : (
          <Text size="sm">{`Prescritto il ${formatDate(row.date)}`}</Text>
        )
      }
      actions={
        <IconAction
          label="Aggiorna"
          icon={RefreshCw}
          loading={starting || data.refreshing}
          onClick={() => {
            void refreshLists();
          }}
        />
      }
      tabs={[
        {
          value: "prescrizione",
          label: "Prescrizione",
          content: (
            <Paper withBorder p="sm" radius="md">
              <DetailSection
                title="Prescrizione"
                actions={<PdfLink documentRef={row.documentRef} />}
              >
                <DetailField label="Data prescrizione">
                  {formatDate(row.date)}
                </DetailField>
                <DetailField label="NRE">
                  <CopyButton value={row.nre} label="Copia NRE" />
                </DetailField>
              </DetailSection>
            </Paper>
          ),
        },
      ]}
      onClose={onClose}
    />
  );
}
