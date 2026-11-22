import { Alert, Group, Loader, Modal, Stack } from "@mantine/core";
import { useMediaQuery } from "@mantine/hooks";
import { notifications } from "@mantine/notifications";
import { RefreshCw } from "lucide-react";
import { useState } from "react";
import type { JSX } from "react";

import { CopyButton } from "../../components/CopyButton";
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
  const fullScreen = useMediaQuery("(max-width: 48em)");
  const { data, error, notFound, loading, reload } = useRowQuery(
    "/api/drugs/row",
    { nre },
  );
  const [starting, setStarting] = useState(false);
  const row = data?.row;

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

  let title = "Carico il farmaco…";
  if (row !== undefined) {
    title = row.title;
  }
  if (notFound) {
    title = "Farmaco non trovato";
  }

  return (
    <Modal
      opened
      onClose={onClose}
      title={title}
      fullScreen={fullScreen}
      centered
    >
      {loading && row === undefined ? <Loader size="sm" /> : null}
      {error === undefined ? null : (
        <Alert color="red" role="alert">
          Il portale regionale non risponde. Riprova tra poco.
        </Alert>
      )}
      {row === undefined ? null : (
        <Stack gap="lg">
          <DetailSection title="Stato">
            <StatusBadge tone={row.status.tone}>{row.status.label}</StatusBadge>
          </DetailSection>
          <DetailSection title="Prescrizione">
            <DetailField label="Data">{formatDate(row.date)}</DetailField>
            <DetailField label="NRE">
              <CopyButton value={row.nre} label="Copia NRE" />
            </DetailField>
          </DetailSection>
          <DetailSection title="Azioni">
            <Group gap="xs">
              <PdfLink documentRef={row.documentRef} />
              <IconAction
                label="Aggiorna"
                icon={RefreshCw}
                loading={starting || data?.refreshing === true}
                onClick={() => {
                  void refreshLists();
                }}
              />
            </Group>
          </DetailSection>
        </Stack>
      )}
    </Modal>
  );
}
